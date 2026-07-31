package server

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/msvens/mimage/img"
	"github.com/msvens/mphotos/internal/config"
	"github.com/msvens/mphotos/internal/gdrive"
	"go.uber.org/zap"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
)

// avatarSizes are the square-cropped variants generated for each guest avatar.
var avatarSizes = []img.Options{
	img.NewOptions(img.ResizeAndCrop, 48, 48, false),
	img.NewOptions(img.ResizeAndCrop, 192, 192, false),
}

const maxAvatarBytes = 10 << 20 // 10 MB

// uploadGuestAvatarFromFile stores an avatar from a multipart file upload.
func (s *mserver) uploadGuestAvatarFromFile(r *http.Request, guestId uuid.UUID) (interface{}, error) {
	if err := r.ParseMultipartForm(maxAvatarBytes); err != nil {
		return nil, err
	}
	file, _, err := r.FormFile("avatar")
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	return s.setGuestAvatar(guestId, file)
}

// uploadGuestAvatarFromURL stores an avatar fetched from a remote URL. The fetch
// is SSRF-guarded and size-capped since the URL is guest-supplied.
func (s *mserver) uploadGuestAvatarFromURL(r *http.Request, guestId uuid.UUID) (interface{}, error) {
	type request struct {
		Url string
	}
	var params request
	if err := decodeRequest(r, &params); err != nil {
		return nil, err
	}
	data, err := fetchRemoteImage(params.Url)
	if err != nil {
		return nil, err
	}
	return s.setGuestAvatar(guestId, bytes.NewReader(data))
}

// deleteGuestAvatar removes a guest's avatar files and clears the reference.
func (s *mserver) deleteGuestAvatar(r *http.Request, guestId uuid.UUID) (interface{}, error) {
	guest, err := s.pg.Guest.Get(guestId)
	if err != nil {
		return nil, err
	}
	s.removeAvatarFiles(guestId, guest.Avatar)
	return s.pg.Guest.UpdateAvatar("", guestId)
}

// handleGuestAvatar serves a guest's avatar (original or a whitelisted size). It
// is public and keyed by guest id, so it is a raw handler, not JSON-wrapped.
func (s *mserver) handleGuestAvatar(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("guestid")
	gid, err := uuid.Parse(id)
	if err != nil {
		http.Error(w, "Invalid guest id", http.StatusNotFound)
		return
	}
	guest, err := s.pg.Guest.Get(gid)
	if err != nil || guest.Avatar == "" {
		http.Error(w, "No avatar", http.StatusNotFound)
		return
	}
	fname := id + guest.Avatar
	if size := r.PathValue("size"); size != "" {
		switch size {
		case "48", "192":
			fname = fmt.Sprint(id, "-", size, guest.Avatar)
		default:
			http.Error(w, "No Such Image Size", http.StatusNotFound)
			return
		}
	}
	http.ServeFile(w, r, config.AvatarFilePath(fname))
}

// setGuestAvatar stores a new avatar image and records its extension. It writes
// the new files first, then removes stale files only if the extension changed,
// so a failed store never destroys the existing avatar.
func (s *mserver) setGuestAvatar(guestId uuid.UUID, r io.Reader) (interface{}, error) {
	prev, err := s.pg.Guest.Get(guestId)
	if err != nil {
		return nil, err
	}
	ext, err := storeGuestAvatar(guestId, r)
	if err != nil {
		return nil, err
	}
	if prev.Avatar != "" && prev.Avatar != ext {
		s.removeAvatarFiles(guestId, prev.Avatar)
	}
	return s.pg.Guest.UpdateAvatar(ext, guestId)
}

// storeGuestAvatar sniffs the image type (jpeg/png only, from the bytes — never a
// caller-supplied header), writes the original to the avatar dir, and generates
// the square size variants. It returns the stored file extension.
func storeGuestAvatar(guestId uuid.UUID, r io.Reader) (string, error) {
	buff := make([]byte, 512)
	n, err := io.ReadFull(r, buff)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", err
	}
	buff = buff[:n]
	mt := http.DetectContentType(buff)
	if mt != gdrive.Jpeg && mt != gdrive.Png {
		return "", BadRequestError("avatar must be a jpeg or png image")
	}
	ext := ".jpg"
	if mt == gdrive.Png {
		ext = ".png"
	}
	id := guestId.String()
	fileName := config.AvatarFilePath(id + ext)
	dst, err := os.Create(fileName)
	if err != nil {
		return "", err
	}
	defer func() { _ = dst.Close() }()
	if _, err := io.Copy(dst, io.MultiReader(bytes.NewReader(buff), r)); err != nil {
		return "", err
	}
	sizes := make(map[string]img.Options)
	for _, size := range avatarSizes {
		sizes[config.AvatarFilePath(fmt.Sprint(id, "-", size.Width, ext))] = size
	}
	if err := img.TransformFile(fileName, sizes); err != nil {
		return "", err
	}
	return ext, nil
}

// removeAvatarFiles deletes a guest's avatar original + size variants for the
// given extension, tolerating already-missing files. Best-effort.
func (s *mserver) removeAvatarFiles(guestId uuid.UUID, ext string) {
	if ext == "" {
		return
	}
	id := guestId.String()
	names := []string{id + ext}
	for _, size := range avatarSizes {
		names = append(names, fmt.Sprint(id, "-", size.Width, ext))
	}
	for _, n := range names {
		if err := os.Remove(config.AvatarFilePath(n)); err != nil && !os.IsNotExist(err) {
			s.l.Warnw("could not remove avatar file", "file", n, zap.Error(err))
		}
	}
}

// fetchRemoteImage GETs a guest-supplied image URL with basic SSRF protection
// (http/https only; the resolved host must not be loopback/private/link-local)
// and caps the download size.
func fetchRemoteImage(rawurl string) ([]byte, error) {
	if err := validatePublicURL(rawurl); err != nil {
		return nil, err
	}
	resp, err := http.Get(rawurl)
	if err != nil {
		return nil, BadRequestError("could not fetch url: " + err.Error())
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, BadRequestError(fmt.Sprintf("url returned status %d", resp.StatusCode))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxAvatarBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxAvatarBytes {
		return nil, BadRequestError("image exceeds the size limit")
	}
	return data, nil
}

// validatePublicURL rejects non-http(s) schemes and URLs whose host resolves to
// a loopback/private/link-local/unspecified address. Best-effort SSRF guard (it
// does not defend against DNS rebinding) — adequate for a hobby guest endpoint.
func validatePublicURL(rawurl string) error {
	u, err := url.Parse(rawurl)
	if err != nil {
		return BadRequestError("invalid url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return BadRequestError("url must be http or https")
	}
	host := u.Hostname()
	if host == "" {
		return BadRequestError("url has no host")
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return BadRequestError("could not resolve url host")
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
			ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return BadRequestError("url resolves to a disallowed address")
		}
	}
	return nil
}

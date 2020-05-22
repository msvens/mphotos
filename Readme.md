# About

mphotos is the backend service for [mphotos-app](https://www.github.com/msvens/mphotos-app).
mphotos exposes and api for working with images and is tightly integrated with google drive.

**Goal**: *Once your images have been upload to your google drive they should be accessible through your website*
## Features
- Create a long lived connection to a remote google drive folder containing pictures
- (Automatic) download of new images to local storage
- Full automation of
  - Image information extraction using [exiftool](https://sno.phy.queensu.ca/~phil/exiftool/)
  - Thumbnail creation using [libvips](https://libvips.github.io/libvips/)
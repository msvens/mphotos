package gdrive

import (
	"google.golang.org/api/googleapi"
	"strconv"
	"strings"
)

//Common mime types
const (
	False = "false"
	True  = "true"
	Jpeg  = "image/jpeg"
	Gif   = "image/gif"
	Png   = "image/png"

	Folder  = "application/vnd.google-apps.folder"
	Sheet   = "application/vnd.google-apps.spreadsheet"
	Doc     = "application/vnd.google-apps.document"
	Present = "application/vnd.google-apps.presentation"
)

const (
	InitState = iota
	TermState
	OpState
	NotState
)

type Query struct {
	buff   strings.Builder
	err    *googleapi.Error
	state  int
	nextOp []string
}

func NewQuery() *Query {
	return &Query{state: InitState}
}

func (q *Query) Reset() {
	q.state = InitState
	q.err = nil
	q.buff.Reset()
	q.nextOp = nil
}

func escape(q string) string {
	return strings.Replace(q, "'", "\\'", -1)
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func (q *Query) String() string {
	return q.buff.String()
}

func (q *Query) IsEmpty() bool {
	return q.buff.Len() == 0
}

func (q *Query) Err() *googleapi.Error {
	return q.err
}

//Terms
func (q *Query) setTextTerm(t string, nextOp []string) *Query {
	if q.state != InitState && q.state != NotState {
		q.err = &googleapi.Error{Code: ErrorBadRequest, Message: "Wrong query state"}
		return q
	}
	q.state = TermState
	q.nextOp = nextOp
	q.buff.WriteString(t)
	q.buff.WriteString(" ")
	return q
}

func (q *Query) TrashedEq(b bool) *Query {
	if q.state == OpState {
		q.And()
	}
	q.buff.WriteString("trashed = ")
	q.buff.WriteString(strconv.FormatBool(b))
	q.state = OpState
	return q
}

func (q *Query) MimeType() *Query {
	return q.setTextTerm("mimeType", []string{"=", "!=", "contains"})
}

func (q *Query) Name() *Query {
	return q.setTextTerm("name", []string{"=", "!=", "contains"})
}

func (q *Query) Parents() *Query {
	return q.setTextTerm("parents", []string{"in"})
}

//Operands
func (q *Query) writeOp(op string, value string) *Query {
	if q.state != TermState {
		q.err = &googleapi.Error{Code: ErrorBadRequest, Message: "Wrong query state"}
		return q
	}
	if !contains(q.nextOp, op) {
		q.err = &googleapi.Error{Code: ErrorBadRequest, Message: "term did not expect operand: " + op}
		return q
	}
	q.state = OpState
	q.nextOp = nil
	q.buff.WriteString(op)
	q.buff.WriteString(" '")
	q.buff.WriteString(escape(value))
	q.buff.WriteString("'")
	return q
}

func (q *Query) Eq(value string) *Query {
	return q.writeOp("=", value)

}
func (q *Query) NotEq(value string) *Query {
	return q.writeOp("!=", value)
}
func (q *Query) Contains(value string) *Query {
	return q.writeOp("contains", value)
}
func (q *Query) In(value string) *Query {
	return q.writeOp("in", value)
}

func (q *Query) And() *Query {
	if q.state != OpState {
		q.err = &googleapi.Error{Code: ErrorBadRequest, Message: "Wrong query state"}
		return q
	}
	q.state = InitState
	q.buff.WriteString(" and ")
	return q
}

func (q *Query) Or() *Query {
	if q.state != OpState {
		q.err = &googleapi.Error{Code: ErrorBadRequest, Message: "Wrong query state"}
		return q
	}
	q.state = InitState
	q.buff.WriteString(" or ")
	return q
}

func (q *Query) Not() *Query {
	if q.state != InitState {
		q.err = &googleapi.Error{Code: ErrorBadRequest, Message: "Wrong query state"}
		return q
	}
	q.buff.WriteString("not ")
	q.state = NotState
	return q
}

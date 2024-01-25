package clog

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/apex/log"
)

type Handler struct {
	mu     sync.Mutex
	Writer io.WriteCloser
}

var levelToStrings = [...]string{
	log.DebugLevel: "DEBUG",
	log.InfoLevel:  "INFO",
	log.WarnLevel:  "WARN",
	log.ErrorLevel: "ERROR",
	log.FatalLevel: "FATAL",
}

// field used for sorting.
type field struct {
	Name  string
	Value interface{}
}

// by sorts fields by name.
type byName []field

func (a byName) Len() int           { return len(a) }
func (a byName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byName) Less(i, j int) bool { return a[i].Name < a[j].Name }

func NewHandler(w io.WriteCloser) *Handler {
	return &Handler{Writer: w}
}

func (h *Handler) SetOutput(w io.WriteCloser) {
	h.mu.Lock()
	defer h.mu.Unlock()

	_ = h.Writer.Close()
	h.Writer = w
}

func (h *Handler) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.Writer == nil {
		return
	}

	if h.Writer == os.Stdout || h.Writer == os.Stderr {
		return
	}

	// Only close if Writer points to a file
	_ = h.Writer.Close()
}

func (h *Handler) HandleLog(e *log.Entry) error {
	level := levelToStrings[e.Level]
	var fields []field

	for k, v := range e.Fields {
		fields = append(fields, field{k, v})
	}

	sort.Sort(byName(fields))

	now := time.Now()
	var b bytes.Buffer
	_, _ = fmt.Fprintf(&b, "%5s %s %-25s", level, now.Format(time.DateTime), e.Message)

	for _, f := range fields {
		_, _ = fmt.Fprintf(&b, " %s=%v", f.Name, f.Value)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	_, _ = fmt.Fprintln(h.Writer, b.String())

	return nil
}

package main

import (
	"bytes"
	"compress/flate"
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	flag "github.com/spf13/pflag"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
)

//go:embed template.html
var templateString string

const (
	minIterations = 100_000
	saltLen       = 32
	keyLen        = 32
)

func main() {
	err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var errNoInput = errors.New("missing input file")

func run() error {
	var displayedFilename = ""
	var displayedMimetype = ""
	var download = false

	var iterations = 1_000_000
	var password = ""
	var noCompression = false
	var noMinify = false

	flag.BoolVar(&noMinify, "no-minify", noMinify, "Don't minify output")
	flag.BoolVar(&noCompression, "no-compression", noCompression, "Don't compress body")
	flag.BoolVarP(&download, "download", "d", download, "Download content after decryption")
	flag.IntVarP(&iterations, "iterations", "i", iterations, "PBKDF2 iteration count")
	flag.StringVarP(&password, "password", "p", password, "Password (default: randomly generated)")
	flag.StringVar(&displayedFilename, "filename", displayedFilename, "Filename displayed when downloading (default: input filename)")
	flag.StringVar(&displayedMimetype, "mime-type", displayedMimetype, "Mimetype displayed when downloading (default: guess from inputs)")
	flag.Parse()

	inPath := flag.Arg(0)
	if inPath == "" {
		flag.Usage()
		return errNoInput
	}

	if iterations < minIterations {
		return fmt.Errorf("need at least %d iterations", minIterations)
	}

	if password == "" {
		password = rand.Text()
		fmt.Fprintf(os.Stderr, "Password: %s\n", password)
	}

	if displayedFilename == "" {
		displayedFilename = filepath.Base(inPath)
	}

	tmpl := template.Must(template.New("template.html").
		Funcs(template.FuncMap{"base64": base64.StdEncoding.EncodeToString}).
		Parse(templateString))

	body, err := os.ReadFile(inPath)
	if err != nil {
		return fmt.Errorf("read input file: %w", err)
	}

	if displayedMimetype == "" {
		displayedMimetype = http.DetectContentType(body)
	}

	h := header{
		Filename: displayedFilename,
		MimeType: displayedMimetype,
		Size:     len(body),
		Download: download,
	}

	if !noCompression {
		body, h.Compression, err = compress(body)
		if err != nil {
			return fmt.Errorf("compress body: %w", err)
		}
	}

	plaintext, err := preparePlaintext(&h, body)
	if err != nil {
		return fmt.Errorf("prepare plaintext: %w", err)
	}

	envelope, err := encrypt(plaintext, password, iterations)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	outPath := inPath + ".enc.html"

	o, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("open output file: %w", err)
	}
	defer o.Close()

	var w io.WriteCloser = o
	if !noMinify {
		m := newMinifier()
		w = m.Writer("text/html", o)
	}

	err = tmpl.Execute(w, envelope)
	if err != nil {
		os.Remove(outPath)
		return fmt.Errorf("execute template: %w", err)
	}

	err = w.Close()
	if err != nil {
		os.Remove(outPath)
		return fmt.Errorf("close writer: %w", err)
	}

	fmt.Printf("Written: %s => %s\n", inPath, outPath)

	return nil
}

func newMinifier() *minify.M {
	m := minify.New()
	m.AddFunc("text/css", css.Minify)
	m.AddFunc("text/html", html.Minify)
	m.AddFuncRegexp(regexp.MustCompile("^(application|text)/(x-)?(java|ecma)script$"), js.Minify)
	return m
}

func compress(body []byte) ([]byte, string, error) {
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.BestCompression)
	if err != nil {
		return nil, "", err
	}
	if _, err := w.Write(body); err != nil {
		return nil, "", err
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	if buf.Len() >= len(body) {
		return body, "", nil // not worth it
	}
	return buf.Bytes(), "deflate-raw", nil
}

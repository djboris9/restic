package webdav

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"restic"
	"strings"

	"encoding/xml"
	"restic/backend"
	"restic/errors"
)

const connLimit = 10

// restPath returns the path to the given resource.
func restPath(url *url.URL, h restic.Handle) string {
	u := *url

	var dir string

	switch h.Type {
	case restic.ConfigFile:
		dir = ""
		h.Name = "config"
	case restic.DataFile:
		dir = backend.Paths.Data
	case restic.SnapshotFile:
		dir = backend.Paths.Snapshots
	case restic.IndexFile:
		dir = backend.Paths.Index
	case restic.LockFile:
		dir = backend.Paths.Locks
	case restic.KeyFile:
		dir = backend.Paths.Keys
	default:
		dir = string(h.Type)
	}

	u.Path = path.Join(url.Path, dir, h.Name)

	return u.String()
}

type webdavBackend struct {
	url      *url.URL
	connChan chan struct{}
	client   http.Client
}

// Open opens the REST backend with the given config.
func Open(cfg Config) (restic.Backend, error) {
	connChan := make(chan struct{}, connLimit)
	for i := 0; i < connLimit; i++ {
		connChan <- struct{}{}
	}
	tr := &http.Transport{}
	client := http.Client{Transport: tr}

	return &webdavBackend{url: cfg.URL, connChan: connChan, client: client}, nil
}

// Location returns this backend's location (the server's URL).
func (b *webdavBackend) Location() string {
	return b.url.String()
}

// Load returns the data stored in the backend for h at the given offset
// and saves it in p. Load has the same semantics as io.ReaderAt.
func (b *webdavBackend) Load(h restic.Handle, p []byte, off int64) (n int, err error) {
	fmt.Println("Load")
	if err := h.Valid(); err != nil {
		return 0, err
	}

	// invert offset
	if off < 0 {
		info, err := b.Stat(h)
		if err != nil {
			return 0, errors.Wrap(err, "Stat")
		}

		if -off > info.Size {
			off = 0
		} else {
			off = info.Size + off
		}
	}

	req, err := http.NewRequest("GET", restPath(b.url, h), nil)
	if err != nil {
		return 0, errors.Wrap(err, "http.NewRequest")
	}
	req.Header.Add("Range", fmt.Sprintf("bytes=%d-%d", off, off+int64(len(p))))
	<-b.connChan
	resp, err := b.client.Do(req)
	b.connChan <- struct{}{}

	if resp != nil {
		defer func() {
			e := resp.Body.Close()

			if err == nil {
				err = errors.Wrap(e, "Close")
			}
		}()
	}

	if err != nil {
		return 0, errors.Wrap(err, "client.Do")
	}
	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		return 0, errors.Errorf("Load: unexpected HTTP response code %v", resp.StatusCode)
	}

	return io.ReadFull(resp.Body, p)
}

// Save stores data in the backend at the handle.
func (b *webdavBackend) Save(h restic.Handle, p []byte) (err error) {
	fmt.Println("Save")
	if err := h.Valid(); err != nil {
		return err
	}

	path := restPath(b.url, h)
	req, err := http.NewRequest("PUT", path, bytes.NewReader(p))
	if err != nil {
		return errors.Wrap(err, "http.NewRequest")
	}
	req.Header.Add("Translate", "f")
	<-b.connChan
	resp, err := b.client.Do(req)
	b.connChan <- struct{}{}

	if resp != nil {
		defer func() {
			e := resp.Body.Close()

			if err == nil {
				err = errors.Wrap(e, "Close")
			}
		}()
	}

	if err != nil {
		return errors.Wrap(err, "client.Post")
	}

	if resp.StatusCode != http.StatusCreated {
		return errors.Errorf("Save: unexpected HTTP response code %v for %v", resp.StatusCode, path)
	}

	return nil
}

// Stat returns information about a blob.
func (b *webdavBackend) Stat(h restic.Handle) (restic.FileInfo, error) {
	fmt.Println("Head")
	if err := h.Valid(); err != nil {
		return restic.FileInfo{}, err
	}

	<-b.connChan
	resp, err := b.client.Head(restPath(b.url, h))
	b.connChan <- struct{}{}
	if err != nil {
		return restic.FileInfo{}, errors.Wrap(err, "client.Head")
	}

	if err = resp.Body.Close(); err != nil {
		return restic.FileInfo{}, errors.Wrap(err, "Close")
	}

	if resp.StatusCode != 200 {
		return restic.FileInfo{}, errors.Errorf("Stat: unexpected HTTP response code %v", resp.StatusCode)
	}

	if resp.ContentLength < 0 {
		return restic.FileInfo{}, errors.New("negative content length")
	}

	bi := restic.FileInfo{
		Size: resp.ContentLength,
	}

	return bi, nil
}

// Test returns true if a blob of the given type and name exists in the backend.
func (b *webdavBackend) Test(t restic.FileType, name string) (bool, error) {
	fmt.Println("Test")
	_, err := b.Stat(restic.Handle{Type: t, Name: name})
	if err != nil {
		return false, nil
	}

	return true, nil
}

// Remove removes the blob with the given name and type.
func (b *webdavBackend) Remove(t restic.FileType, name string) error {
	fmt.Println("Remove")
	h := restic.Handle{Type: t, Name: name}
	if err := h.Valid(); err != nil {
		return err
	}

	req, err := http.NewRequest("DELETE", restPath(b.url, h), nil)
	if err != nil {
		return errors.Wrap(err, "http.NewRequest")
	}
	<-b.connChan
	resp, err := b.client.Do(req)
	b.connChan <- struct{}{}

	if err != nil {
		return errors.Wrap(err, "client.Do")
	}

	if resp.StatusCode != 200 {
		return errors.New("blob not removed")
	}

	return resp.Body.Close()
}

// List returns a channel that yields all names of blobs of type t. A
// goroutine is started for this. If the channel done is closed, sending
// stops.
func (b *webdavBackend) List(t restic.FileType, done <-chan struct{}) <-chan string {
	fmt.Println("List")
	ch := make(chan string)

	url := restPath(b.url, restic.Handle{Type: t})
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}

	req, err := http.NewRequest("PROPFIND", url, nil)
	req.Header.Add("Depth", "0")
	if err != nil {
		fmt.Printf("Error %v\n", err)
		close(ch)
		return ch
	}

	<-b.connChan
	resp, err := b.client.Do(req)
	b.connChan <- struct{}{}

	if resp != nil {
		defer resp.Body.Close()
	}

	if err != nil {
		fmt.Printf("Error %v\n", err)
		close(ch)
		return ch
	}

	//	content, _ := ioutil.ReadAll(resp.Body)
	//	fmt.Println(string(content))

	var list Multistatus
	if err = xml.NewDecoder(resp.Body).Decode(&list); err != nil {
		fmt.Printf("Error %v\n", err)
		close(ch)
		return ch
	}
	fmt.Println(list)

	go func() {
		defer close(ch)
		for _, m := range list.Href {
			select {
			case ch <- m:
			case <-done:
				return
			}
		}
	}()

	return ch
}

// Close closes all open files.
func (b *webdavBackend) Close() error {
	// this does not need to do anything, all open files are closed within the
	// same function.
	return nil
}

package catalog

import (
	"bytes"
	"context"
	"crypto/des"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func encryptFixture(data []byte, inner bool) []byte {
	pad := func(b []byte) []byte { n := 8 - len(b)%8; return append(b, bytes.Repeat([]byte{byte(n)}, n)...) }
	if inner {
		data = pad(data)
	}
	data = pad(data)
	c, _ := des.NewCipher([]byte(directoryKey))
	for i := 0; i < len(data); i += 8 {
		c.Encrypt(data[i:i+8], data[i:i+8])
	}
	out := make([]byte, hex.EncodedLen(len(data)))
	hex.Encode(out, data)
	return out
}
func newStore(t *testing.T) *store {
	path := filepath.Join(t.TempDir(), "globalspeed", "serverlist.json")
	return &store{path: func() (string, error) { return path, nil }}
}
func indexClient(cipher []byte, digest string) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := cipher
		if r.URL.Host == "index.example" {
			body, _ = json.Marshal([]map[string]string{{"name": "serverlist_encrypt_url", "filename": "https://down.cnspeedtest.cn:8043/TaierAndroid/Config/serverlist_encrypt.json", "md5": digest}})
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body))}, nil
	})}
}
func TestOriginalEncryptedSnapshot(t *testing.T) {
	cipher, _ := assets.ReadFile("serverlist_encrypt.json")
	rows, err := decodeDirectory(cipher)
	if err != nil || len(rows) != 606 {
		t.Fatalf("rows=%d err=%v", len(rows), err)
	}
	// Capture the vendor's original cipher bytes, rather than reserializing the JSON.
	if hash(cipher) != "6d492c0d5d07e790ac8e609097879b74" {
		t.Fatal("snapshot bytes changed")
	}
	for _, inner := range []bool{false, true} {
		data, _ := json.Marshal(rows[:1])
		for padding := 0; padding < 8; padding++ {
			plain := append(append([]byte(nil), data...), bytes.Repeat([]byte{' '}, padding)...)
			decoded, err := decodeDirectory(encryptFixture(plain, inner))
			if err != nil || len(decoded) != 1 {
				t.Fatalf("inner=%v padding=%d err=%v", inner, padding, err)
			}
		}
	}
}
func TestUpdateWritesOriginalCipherAndLoadsCache(t *testing.T) {
	s := newStore(t)
	data := encryptFixture([]byte(`[{"hostid":"new-node","hostname":"New server","hostip":"203.0.113.8","port":"65499","pname":"江苏"}]`), false)
	result, err := s.update(context.Background(), indexClient(data, hash(data)), "https://index.example/config")
	if err != nil || !result.Updated || result.Count != 1 {
		t.Fatalf("%+v %v", result, err)
	}
	saved, _ := os.ReadFile(result.Path)
	if !bytes.Equal(saved, data) {
		t.Fatal("saved format changed")
	}
	restarted := &store{path: s.path}
	if rows := restarted.servers(); len(rows) != 1 || rows[0].ID != "new-node" {
		t.Fatal(rows)
	}
	result, err = s.update(context.Background(), indexClient(data, hash(data)), "https://index.example/config")
	if err != nil || result.Updated {
		t.Fatalf("unchanged update: %+v %v", result, err)
	}
}
func TestFailedUpdatePreservesLastGoodCache(t *testing.T) {
	s := newStore(t)
	original, _ := assets.ReadFile("serverlist_encrypt.json")
	path, _ := s.path()
	if err := writeCache(path, original); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		data   []byte
		digest string
	}{{[]byte("corrupted"), hash(original)}, {[]byte("corrupted"), hash([]byte("corrupted"))}} {
		// Force a changed MD5 for the mismatch case so a download is attempted.
		if tc.digest == hash(original) {
			tc.digest = hash([]byte("different"))
		}
		if _, err := s.update(context.Background(), indexClient(tc.data, tc.digest), "https://index.example/config"); err == nil {
			t.Fatal("invalid update accepted")
		}
		actual, _ := os.ReadFile(path)
		if !bytes.Equal(original, actual) || len(s.servers()) != 606 {
			t.Fatal("last good cache replaced")
		}
	}
}
func TestInvalidCacheFallsBackAndRejectsUnsafeIndex(t *testing.T) {
	s := newStore(t)
	path, _ := s.path()
	writeCache(path, []byte("not cipher"))
	if len(s.servers()) != 606 || s.result(false).Source != "embedded" {
		t.Fatal("fallback failed")
	}
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(`[{"name":"serverlist_encrypt_url","filename":"http://127.0.0.1/file","md5":"6d492c0d5d07e790ac8e609097879b74"}]`))}, nil
	})}
	if _, err := s.update(context.Background(), client, "https://index.example/config"); err == nil {
		t.Fatal("unsafe download accepted")
	}
}

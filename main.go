package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
)

type FileHash struct {
	Key     []byte
	KeySize int
}

type Server struct {
	fileHash *FileHash
	mux      *http.ServeMux
	db       *Db
}

type FileResp struct {
	Oid  string `json:"oid"`
	Size int    `json:"size"`
}

type Db struct {
	fileStorage map[string][]byte
}

// Gets the []byte of the content of the file and
// pointer to the FileHash to generate oid
func (d *Db) AddFile(f []byte, repo string, fileHash *FileHash) (string, error) {
	oid, err := fileHash.createOid(f)
	if err != nil {
		return "", err
	}
	_, err = d.SelectFile(oid, repo)
	if err == nil {
		return "", errors.New("file already exist in this repo")
	}

	fileKey := repo + ":" + oid

	d.fileStorage[fileKey] = f
	return oid, nil
}

func (d *Db) SelectFile(oid, repo string) ([]byte, error) {
	fileKey := repo + ":" + oid
	file, prs := d.fileStorage[fileKey]
	if !prs {
		return nil, errors.New("no file with that oid")
	}

	return file, nil
}

func (s *Server) HandlePutStoreFile(w http.ResponseWriter, r *http.Request) {
	repo := strings.TrimSpace(r.PathValue("repo"))
	if repo == "" {
		http.Error(w, "invalid repo name", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	oid, err := s.db.AddFile(body, repo, s.fileHash)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(FileResp{Oid: oid, Size: len(body)})
}

func (s *Server) HandleGetFile(w http.ResponseWriter, r *http.Request) {
	// Additional checks for repo and oid validation
	repo := r.PathValue("repo")
	oid := r.PathValue("oid")

	file, err := s.db.SelectFile(string(oid), repo)
	if err != nil {
		log.Println(err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(file)
}

// Generate a 32 byte key for AES hash
func (f *FileHash) generateKey() error {
	key := make([]byte, f.KeySize)
	_, err := rand.Read(key)
	if err != nil {
		return err
	}

	f.Key = key
	return nil
}

func (f *FileHash) createOid(fileContent []byte) (string, error) {
	hasher := sha256.New()
	hasher.Write(fileContent)
	hash := hasher.Sum(nil)

	// To be compatible for oid in URL
	return base64.RawURLEncoding.EncodeToString(hash), nil
}

func main() {
	fh := &FileHash{KeySize: 32}
	err := fh.generateKey()
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()

	server := Server{
		fileHash: fh,
		mux:      mux,
		db:       &Db{fileStorage: make(map[string][]byte)},
	}

	server.mux.HandleFunc("GET /store/{repo}/{oid}", server.HandleGetFile)
	server.mux.HandleFunc("PUT /{repo}", server.HandlePutStoreFile)

	log.Println("listening to port 1234...")
	http.ListenAndServe(":1234", server.mux)
}

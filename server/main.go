package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/feedbooks/webpub-streamer/fetcher"
	"github.com/feedbooks/webpub-streamer/models"
	"github.com/feedbooks/webpub-streamer/parser"
	"github.com/gorilla/mux"
	"github.com/urfave/negroni"
)

type currentBook struct {
	filename    string
	publication models.Publication
	timestamp   time.Time
}

var currentBookList []currentBook

// Serv TODO add doc
func main() {

	filename := os.Args[1]

	n := negroni.Classic()
	n.Use(negroni.NewStatic(http.Dir("public")))
	n.UseHandler(bookHandler(false))

	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		panic(err)
	}

	freePort := l.Addr().(*net.TCPAddr).Port
	l.Close()

	s := &http.Server{
		Handler:        n,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
		Addr:           "localhost:" + strconv.Itoa(freePort),
	}

	filenamePath := base64.StdEncoding.EncodeToString([]byte(filename))
	fmt.Println("http://localhost:" + strconv.Itoa(freePort) + "/" + filenamePath + "/manifest.json")

	log.Fatal(s.ListenAndServe())
}

func bookHandler(test bool) http.Handler {
	serv := mux.NewRouter()

	serv.HandleFunc("/viewer.js", viewer)
	serv.HandleFunc("/sw.js", sw)
	serv.HandleFunc("/{filename}/", bookIndex)
	serv.HandleFunc("/{filename}/manifest.json", getManifest)
	serv.HandleFunc("/{filename}/index.html", bookIndex)
	serv.HandleFunc("/{filename}/{asset:.*}", getAsset)
	return serv
}

func getManifest(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	filename := vars["filename"]

	publication := getPublication(filename, req)

	j, _ := json.Marshal(publication)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(j)
	return
}

func getAsset(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	assetname := vars["asset"]

	publication := getPublication(vars["filename"], req)
	epubReader, mediaType := fetcher.Fetch(publication, assetname)

	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	http.ServeContent(w, req, assetname, time.Now(), epubReader)
	return

}

func bookIndex(w http.ResponseWriter, req *http.Request) {
	var err error

	vars := mux.Vars(req)
	filename := "books/" + vars["filename"]

	t, err := template.ParseFiles("index.html") // Parse template file.
	if err != nil {
		fmt.Println(err)
	}
	t.Execute(w, filename) // merge.
}

func viewer(w http.ResponseWriter, req *http.Request) {

	f, _ := os.OpenFile("public/viewer.js", os.O_RDONLY, 666)
	buff, _ := ioutil.ReadAll(f)

	w.Header().Set("Content-Type", "text/javascript")
	w.Write(buff)
}

func sw(w http.ResponseWriter, req *http.Request) {

	f, _ := os.OpenFile("public/sw.js", os.O_RDONLY, 666)
	buff, _ := ioutil.ReadAll(f)

	w.Header().Set("Content-Type", "text/javascript")
	w.Write(buff)
}

func getPublication(filename string, req *http.Request) models.Publication {
	var current currentBook
	var publication models.Publication

	for _, book := range currentBookList {
		if filename == book.filename {
			current = book
		}
	}

	if current.filename == "" {
		manifestURL := "http://" + req.Host + "/" + filename + "/manifest.json"
		filenamePath, _ := base64.StdEncoding.DecodeString(filename)

		publication = parser.Parse(string(filenamePath), manifestURL)
		for _, book := range currentBookList {
			if filename == book.filename {
				current = book
			}
		}

		currentBookList = append(currentBookList, currentBook{filename: base64.StdEncoding.EncodeToString([]byte(filename)), publication: publication, timestamp: time.Now()})
	} else {
		publication = current.publication
	}

	return publication
}

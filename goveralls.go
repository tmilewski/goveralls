// Copyright (c) 2013 Yasuhiro Matsumoto, Jason McVetta.
// This is Free Software,  released under the MIT license.
// See http://mattn.mit-license.org/2013 for details.

// goveralls is a Go client for Coveralls.io.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"code.google.com/p/go-uuid/uuid"
)

/*
	https://coveralls.io/docs/api_reference
*/

var (
	pkg       = flag.String("package", "", "Go package")
	verbose   = flag.Bool("v", false, "Pass '-v' argument to 'gocov test'")
	gocovjson = flag.String("gocovdata", "", "If supplied, use existing gocov.json")
)

// usage supplants package flag's Usage variable
var usage = func() {
	cmd := os.Args[0]
	// fmt.Fprintf(os.Stderr, "Usage of %s:\n", cmd)
	s := "Usage: %s [options] TOKEN\n"
	fmt.Fprintf(os.Stderr, s, cmd)
	flag.PrintDefaults()
}

// A SourceFile represents a source code file and its coverage data for a
// single job.
type SourceFile struct {
	Name     string        `json:"name"`     // File path of this source file
	Source   string        `json:"source"`   // Full source code of this file
	Coverage []interface{} `json:"coverage"` // Requires both nulls and integers
}

// A Job represents the coverage data from a single run of a test suite.
type Job struct {
	RepoToken    string        `json:"repo_token"`
	ServiceJobId string        `json:"service_job_id"`
	ServiceName  string        `json:"service_name"`
	SourceFiles  []*SourceFile `json:"source_files"`
	Git          *Git          `json:"git,omitempty"`
	RunAt        time.Time     `json:"run_at"`
}

// A Response is returned by the Coveralls.io API.
type Response struct {
	Message string `json:"message"`
	URL     string `json:"url"`
	Error   bool   `json:"error"`
}

func getCoverage() []*SourceFile {
	r, err := loadGocov()
	if err != nil {
		log.Fatalf("Error loading gocov results: %v", err)
	}
	rv, err := parseGocov(r)
	if err != nil {
		log.Fatalf("Error parsing gocov: %v", err)
	}
	return rv
}

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)
	//
	// Parse Flags
	//
	flag.Usage = usage
	service := flag.String("service", "goveralls",
		"The CI service or other environment in which the test suite was run. ")
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}
	//
	// Setup PATH environment variable
	//
	paths := filepath.SplitList(os.Getenv("PATH"))
	if goroot := os.Getenv("GOROOT"); goroot != "" {
		paths = append(paths, filepath.Join(goroot, "bin"))
	}
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		for _, path := range filepath.SplitList(gopath) {
			paths = append(paths, filepath.Join(path, "bin"))
		}
	}
	os.Setenv("PATH", strings.Join(paths, string(filepath.ListSeparator)))

	//
	// Initialize Job
	//
	j := Job{
		RunAt:        time.Now(),
		RepoToken:    flag.Arg(0),
		ServiceJobId: uuid.New(),
		Git:          collectGitInfo(),
		SourceFiles:  getCoverage(),
		ServiceName:  *service,
	}

	b, err := json.Marshal(j)
	if err != nil {
		log.Fatal(err)
	}

	if j.RepoToken == "" {
		os.Stdout.Write(b)
		os.Exit(0)
	}

	params := make(url.Values)
	params.Set("json", string(b))
	res, err := http.PostForm("https://coveralls.io/api/v1/jobs", params)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	var response Response
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		log.Fatal(err)
	}
	if response.Error {
		log.Fatal(response.Message)
	}
	fmt.Println(response.Message)
	fmt.Println(response.URL)
}

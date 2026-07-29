// Command snapshot copies a crawled data folder into the server's embedded
// snapshot directory (internal/dataset/data), which `go:embed` bakes into the
// server binary.
//
// By default it writes a slim snapshot: job descriptions are dropped. A global
// crawl produces hundreds of megabytes of descriptions, and embedding those
// would make the binary as large as the data; the slim snapshot keeps the
// single-file server small while still serving the full searchable board. A
// live ./data folder, when present, takes precedence and serves descriptions
// too. Pass -full to embed everything.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"faangjobs/internal/store"
)

func main() {
	src := flag.String("src", "./data", "crawled data directory to snapshot")
	dst := flag.String("dst", "internal/dataset/data", "embedded snapshot directory to write")
	full := flag.Bool("full", false, "keep job descriptions (much larger binary)")
	flag.Parse()

	srcCompanies := filepath.Join(*src, "companies")
	entries, err := os.ReadDir(srcCompanies)
	if err != nil {
		fmt.Printf("no %s to sync — embedded snapshot left unchanged\n", srcCompanies)
		return
	}

	in, err := store.New(*src)
	if err != nil {
		log.Fatalf("open source store: %v", err)
	}
	// Replace the destination wholesale so companies dropped from the registry
	// don't linger in the binary.
	dstCompanies := filepath.Join(*dst, "companies")
	if err := os.RemoveAll(dstCompanies); err != nil {
		log.Fatalf("clear snapshot: %v", err)
	}
	out, err := store.New(*dst)
	if err != nil {
		log.Fatalf("open snapshot store: %v", err)
	}

	companies, jobs := 0, 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		id := e.Name()[:len(e.Name())-len(".json")]
		res, err := in.ReadCompany(id)
		if err != nil {
			log.Fatalf("read %s: %v", id, err)
		}
		if !*full {
			for i := range res.Jobs {
				res.Jobs[i] = res.Jobs[i].Slim()
			}
		}
		if err := out.WriteCompany(*res); err != nil {
			log.Fatalf("write %s: %v", id, err)
		}
		companies++
		jobs += len(res.Jobs)
	}

	if status, err := in.ReadStatus(); err == nil && status != nil {
		if err := out.WriteStatus(*status); err != nil {
			log.Fatalf("write status: %v", err)
		}
	}

	kind := "slim"
	if *full {
		kind = "full"
	}
	fmt.Printf("embedded %s snapshot synced (%d companies, %d jobs)\n", kind, companies, jobs)
}

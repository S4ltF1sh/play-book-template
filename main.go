package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"playbook/internal/server"
	"playbook/internal/store"
)

//go:embed content
var contentFS embed.FS

//go:embed web
var webFS embed.FS

func main() {
	port := flag.Int("port", 0, "port to listen on (default: course.json \"port\", else 4358)")
	host := flag.String("host", "127.0.0.1", "address to bind (0.0.0.0 for Docker/LAN — the playground runs arbitrary code, only expose on trusted networks)")
	dataDir := flag.String("data", "", "directory for user data (default: OS config dir)")
	noOpen := flag.Bool("no-open", false, "do not open the browser automatically")
	flag.Parse()

	appID := "playbook"
	if b, err := contentFS.ReadFile("content/course.json"); err == nil {
		var m struct {
			AppID string `json:"app_id"`
			Port  int    `json:"port"`
		}
		if json.Unmarshal(b, &m) == nil {
			if m.AppID != "" {
				appID = m.AppID
			}
			if *port == 0 && m.Port != 0 {
				*port = m.Port
			}
		}
	}
	if *port == 0 {
		*port = 4358
	}

	if *dataDir == "" {
		cfg, err := os.UserConfigDir()
		if err != nil {
			log.Fatalf("cannot determine config dir: %v (use --data)", err)
		}
		*dataDir = filepath.Join(cfg, appID)
	}
	if err := os.MkdirAll(*dataDir, 0o755); err != nil {
		log.Fatalf("cannot create data dir %s: %v", *dataDir, err)
	}

	db, err := store.Open(filepath.Join(*dataDir, "user.db"))
	if err != nil {
		log.Fatalf("cannot open database: %v", err)
	}
	defer db.Close()

	content, err := fs.Sub(contentFS, "content")
	if err != nil {
		log.Fatal(err)
	}
	web, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}

	srv := server.New(content, web, db)
	addr := fmt.Sprintf("%s:%d", *host, *port)
	url := fmt.Sprintf("http://127.0.0.1:%d", *port)

	if !*noOpen {
		go func() {
			time.Sleep(300 * time.Millisecond)
			openBrowser(url)
		}()
	}

	log.Printf("%s running at %s  (data: %s)", appID, url, *dataDir)
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatal(err)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

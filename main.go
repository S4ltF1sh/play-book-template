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

	"playbook/internal/runner"
	"playbook/internal/server"
	"playbook/internal/store"
)

//go:embed content
var contentFS embed.FS

//go:embed web
var webFS embed.FS

func main() {
	if len(os.Args) > 1 {
		if cmd, ok := subcommands[os.Args[1]]; ok {
			os.Exit(cmd(os.Args[2:]))
		}
	}

	port := flag.Int("port", 0, "port to listen on (default: course.json \"port\", else 4358)")
	host := flag.String("host", "127.0.0.1", "address to bind (0.0.0.0 for Docker/LAN — the playground runs arbitrary code, only expose on trusted networks)")
	dataDir := flag.String("data", "", "directory for user data (default: OS config dir)")
	envDir := flag.String("env-dir", "", "the playbook's private toolchain environment, $ENV_DIR (default: $PLAYBOOK_ENV_DIR, else <data>/env)")
	noOpen := flag.Bool("no-open", false, "do not open the browser automatically")
	flag.Parse()

	appID, coursePort := courseMeta()
	if *port == 0 {
		*port = coursePort
	}
	if *port == 0 {
		*port = 4358
	}

	*dataDir = resolveDataDir(*dataDir, appID)
	db, err := store.Open(filepath.Join(*dataDir, "user.db"))
	if err != nil {
		log.Fatalf("cannot open database: %v", err)
	}
	defer db.Close()

	toolchains := loadToolchains(*dataDir, *envDir)

	content, err := fs.Sub(contentFS, "content")
	if err != nil {
		log.Fatal(err)
	}
	web, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}

	srv := server.New(content, web, db, toolchains)
	addr := fmt.Sprintf("%s:%d", *host, *port)
	url := fmt.Sprintf("http://127.0.0.1:%d", *port)

	if !*noOpen {
		go func() {
			time.Sleep(300 * time.Millisecond)
			openBrowser(url)
		}()
	}

	log.Printf("%s running at %s  (data: %s, env: %s)", appID, url, *dataDir, toolchains.EnvDir())
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatal(err)
	}
}

// courseMeta reads app_id and port from the embedded course.json.
func courseMeta() (appID string, port int) {
	appID = "playbook"
	if b, err := contentFS.ReadFile("content/course.json"); err == nil {
		var m struct {
			AppID string `json:"app_id"`
			Port  int    `json:"port"`
		}
		if json.Unmarshal(b, &m) == nil {
			if m.AppID != "" {
				appID = m.AppID
			}
			port = m.Port
		}
	}
	return appID, port
}

func resolveDataDir(dir, appID string) string {
	if dir == "" {
		cfg, err := os.UserConfigDir()
		if err != nil {
			log.Fatalf("cannot determine config dir: %v (use --data)", err)
		}
		dir = filepath.Join(cfg, appID)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatalf("cannot create data dir %s: %v", dir, err)
	}
	return dir
}

// loadToolchains reads content/toolchains.json. $ENV_DIR precedence:
// --env-dir, then $PLAYBOOK_ENV_DIR (Docker bakes one into the image), then
// <data>/env. Persistent workspaces live in <data>/workspaces.
func loadToolchains(dataDir, envDir string) *runner.Registry {
	if envDir == "" {
		envDir = os.Getenv("PLAYBOOK_ENV_DIR")
	}
	if envDir == "" {
		envDir = filepath.Join(dataDir, "env")
	}
	// commands run inside the workspace, so relative paths would break
	envDir, err := filepath.Abs(envDir)
	if err != nil {
		log.Fatal(err)
	}
	if dataDir, err = filepath.Abs(dataDir); err != nil {
		log.Fatal(err)
	}
	b, err := contentFS.ReadFile("content/toolchains.json")
	if err != nil {
		log.Fatalf("cannot read content/toolchains.json: %v", err)
	}
	reg, err := runner.Load(b, envDir, filepath.Join(dataDir, "workspaces"))
	if err != nil {
		log.Fatal(err)
	}
	return reg
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

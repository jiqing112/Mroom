package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mrome/internal/api"
	"mrome/internal/audiostream"
	"mrome/internal/config"
	"mrome/internal/jellyfin"
	"mrome/internal/plex"
	"mrome/internal/scanner"
	"mrome/internal/store"
	"mrome/internal/subsonic"
)

func main() {
	log.SetFlags(log.Ltime)
	cfg := config.Load()
	if err := cfg.EnsureDirs(); err != nil {
		log.Fatalf("创建数据目录失败: %v", err)
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer st.Close()
	st.SetCoversDir(cfg.CoversDir)

	// The settings panel can toggle guest mode at runtime; its choice is stored in
	// the database and wins over the config file on the next start.
	if v := st.GetSetting("guest_mode", ""); v != "" {
		cfg.GuestMode = v == "1"
	}

	if cfg.GuestMode {
		if g, err := st.EnsureGuest(); err != nil {
			log.Printf("访客模式不可用: %v", err)
		} else {
			log.Printf("免登录访问已开启（访客账号: %s）", g.Username)
		}
	} else if n, _ := st.CountAdmins(); n == 0 {
		if cfg.AdminUser == "" || cfg.AdminPassword == "" {
			// Guest mode was switched off but no administrator exists: nobody could
			// sign in, so fall back instead of locking the instance.
			log.Printf("警告：没有管理员账号，已自动回退到免登录访客模式")
			cfg.GuestMode = true
			_ = st.SetSetting("guest_mode", "1")
			_, _ = st.EnsureGuest()
		} else {
			u, err := st.CreateUser(cfg.AdminUser, cfg.AdminPassword, true)
			if err != nil {
				log.Fatalf("创建管理员失败: %v", err)
			}
			log.Printf("已创建管理员账号: %s / %s （请尽快修改密码）", u.Username, cfg.AdminPassword)
		}
	}

	sc := scanner.New(st, cfg.MusicDir, cfg.CoversDir)
	if cfg.ScanOnStart {
		go func() {
			time.Sleep(400 * time.Millisecond)
			if _, err := sc.Scan(context.Background()); err != nil {
				log.Printf("启动扫描: %v", err)
			}
		}()
	}
	// Periodic rescan so files dropped into the music folder show up without
	// anyone pressing the button. Start() already refuses to run concurrently.
	if cfg.ScanIntervalHours > 0 {
		go func() {
			ticker := time.NewTicker(time.Duration(cfg.ScanIntervalHours) * time.Hour)
			defer ticker.Stop()
			for range ticker.C {
				if sc.Start(context.Background()) {
					log.Printf("定时扫描已开始（每 %d 小时）", cfg.ScanIntervalHours)
				}
			}
		}()
	}

	apiSrv := api.New(st, sc, cfg)
	sub := subsonic.New(st, sc)
	jf := jellyfin.New(st, "")
	emby := jellyfin.New(st, "/emby")
	as := audiostream.New(st)
	px := plex.New(st)
	apiSrv.SetPlexToken(px.Token())

	root := http.NewServeMux()
	root.Handle("/rest/", sub)
	root.Handle("/rest", sub)
	// Jellyfin lives at the root of the site, Emby under /emby; both share one
	// implementation. Audio Station and Plex each own a path prefix.
	for _, p := range jellyfin.MountPaths {
		root.Handle(p, jf)
	}
	root.Handle("/emby/", emby)
	root.Handle("/emby", emby)
	root.Handle("/webapi/", as)
	for _, p := range plex.MountPaths {
		root.Handle(p, px)
	}
	root.Handle("/", apiSrv.Handler())

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           logRequests(root),
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("Mrome 已启动 → http://localhost%s  (Subsonic: http://localhost%s/rest)", cfg.Addr, cfg.Addr)
		log.Printf("音乐目录: %s", cfg.MusicDir)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP 服务异常: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Println("已停止")
}

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) > 5 && r.URL.Path[:5] == "/rest" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, code: 200}
		next.ServeHTTP(sw, r)
		if r.URL.Path != "/api/admin/scan" {
			log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.code, time.Since(start).Round(time.Millisecond))
		}
	})
}

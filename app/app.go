package app

import (
	"context"
	"embed"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx        context.Context
	Clicker    *ClickerService
	FollowSync *FollowSyncService
}

func NewApp(assetFS embed.FS) *App {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}
	dataDir := filepath.Join(configDir, "sg-tools")
	soundDir := filepath.Join(dataDir, "sounds")
	_ = materializeSounds(assetFS, soundDir)
	configPath := filepath.Join(dataDir, "config.json")
	return &App{Clicker: NewClickerService(configPath, soundDir), FollowSync: NewFollowSyncService(configPath)}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.Clicker.setContext(ctx)
	a.FollowSync.setContext(ctx)
	if err := a.Clicker.startup(); err != nil {
		runtime.LogErrorf(ctx, "clicker startup failed: %v", err)
	}
	if err := a.FollowSync.startup(); err != nil {
		runtime.LogErrorf(ctx, "follow-sync startup failed: %v", err)
	}
}

func (a *App) Shutdown(ctx context.Context) {
	a.FollowSync.shutdown()
	a.Clicker.shutdown()
	runtime.LogInfo(ctx, "sg-tools stopped")
}

func materializeSounds(assetFS fs.FS, soundDir string) error {
	if err := os.MkdirAll(soundDir, 0o700); err != nil {
		return err
	}
	for _, name := range []string{"start.wav", "stop.wav"} {
		data, err := fs.ReadFile(assetFS, filepath.ToSlash(filepath.Join("assets", name)))
		if err != nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(soundDir, name), data, 0o600); err != nil {
			return err
		}
	}
	return nil
}

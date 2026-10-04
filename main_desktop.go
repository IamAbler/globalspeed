//go:build desktop || bindings

package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"globalspeed/frontend"
	"globalspeed/internal/catalog"
	"globalspeed/internal/history"
	"globalspeed/internal/speed"
)

type App struct {
	ctx     context.Context
	mu      sync.Mutex
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	closing bool
}

func (a *App) startup(ctx context.Context)      { a.ctx = ctx }
func (a *App) ListServers() []catalog.Server    { return catalog.Servers("", "") }
func (a *App) History() ([]speed.Result, error) { return history.Load() }
func (a *App) StartSpeed(o speed.Options) error {
	if err := o.Validate(); err != nil {
		return err
	}
	if !o.Auto {
		if _, err := catalog.Find(o.ServerID); err != nil {
			return err
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closing {
		return fmt.Errorf("应用正在退出")
	}
	if a.cancel != nil {
		return fmt.Errorf("已有测速任务正在运行")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancel = cancel
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		result, err := speed.NewClient().Run(ctx, o, func(p speed.Progress) { runtime.EventsEmit(a.ctx, "speed:progress", p) })
		if err == nil {
			if saveErr := history.Save(result); saveErr != nil {
				runtime.EventsEmit(a.ctx, "speed:notice", "历史保存失败: "+saveErr.Error())
			}
		}
		a.mu.Lock()
		a.cancel = nil
		a.mu.Unlock()
		cancel()
		if err != nil {
			runtime.EventsEmit(a.ctx, "speed:error", err.Error())
		} else {
			runtime.EventsEmit(a.ctx, "speed:result", result)
		}
	}()
	return nil
}
func (a *App) StopSpeed() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.cancel()
	}
}
func (a *App) shutdown(context.Context) {
	a.mu.Lock()
	a.closing = true
	if a.cancel != nil {
		a.cancel()
	}
	a.mu.Unlock()
	a.wg.Wait()
}
func main() {
	app := &App{}
	err := wails.Run(&options.App{Title: "GlobalSpeed", Width: 960, Height: 680, MinWidth: 800, MinHeight: 560,
		AssetServer: &assetserver.Options{Assets: frontend.Assets}, OnStartup: app.startup, OnShutdown: app.shutdown, Bind: []interface{}{app},
		Windows: &windows.Options{BackdropType: windows.Mica, WebviewIsTransparent: true, WindowIsTranslucent: true},
	})
	if err != nil {
		fmt.Println("桌面程序启动失败:", err)
	}
}

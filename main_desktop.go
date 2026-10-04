//go:build desktop || bindings

package main

import (
	"context"
	_ "embed"
	"fmt"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"globalspeed/frontend"
	"globalspeed/internal/catalog"
	"globalspeed/internal/history"
	"globalspeed/internal/networkinfo"
	"globalspeed/internal/speed"
)

//go:embed build/appicon.png
var appIcon []byte

type App struct {
	ctx             context.Context
	mu              sync.Mutex
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	closing         bool
	selected        map[string]catalog.Server
	selectionCancel context.CancelFunc
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.selected = make(map[string]catalog.Server)
}
func (a *App) ListServers() []catalog.Server    { return catalog.Servers("", "") }
func (a *App) History() ([]speed.Result, error) { return history.Load() }
func (a *App) NetworkInfo() (networkinfo.Info, error) {
	return networkinfo.Lookup(a.ctx, speed.NewClient().HTTP, networkinfo.Endpoint)
}
func (a *App) SelectServer(o speed.MatchOptions) (catalog.Server, error) {
	a.mu.Lock()
	if a.closing {
		a.mu.Unlock()
		return catalog.Server{}, fmt.Errorf("应用正在退出")
	}
	if a.cancel != nil {
		a.mu.Unlock()
		return catalog.Server{}, speed.Failure(992, nil)
	}
	if a.selectionCancel != nil {
		a.selectionCancel()
	}
	ctx, cancel := context.WithTimeout(a.ctx, 45*time.Second)
	a.selectionCancel = cancel
	a.wg.Add(1)
	a.mu.Unlock()
	defer a.wg.Done()
	defer cancel()
	selected, err := speed.NewClient().Match(ctx, o, nil)
	if err != nil {
		return catalog.Server{}, speed.PublicError(err)
	}
	a.mu.Lock()
	a.selected[selected.ID] = selected
	a.mu.Unlock()
	return selected, nil
}
func (a *App) StartSpeed(o speed.Options) error {
	if err := o.Validate(); err != nil {
		return err
	}
	if !o.Auto {
		if _, err := catalog.Find(o.ServerID); err != nil {
			return speed.Failure(139, err)
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closing {
		return fmt.Errorf("应用正在退出")
	}
	if a.cancel != nil {
		return speed.Failure(992, nil)
	}
	var selected *catalog.Server
	if o.Auto {
		server, ok := a.selected[o.ServerID]
		if !ok {
			return speed.Failure(139, nil)
		}
		selected = &server
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancel = cancel
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		client := speed.NewClient()
		progress := func(p speed.Progress) { runtime.EventsEmit(a.ctx, "speed:progress", p) }
		var result speed.Result
		var err error
		if selected != nil {
			result, err = client.RunSelected(ctx, o, *selected, progress)
		} else {
			result, err = client.Run(ctx, o, progress)
		}
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
			runtime.EventsEmit(a.ctx, "speed:error", speed.PublicError(err))
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
	if a.selectionCancel != nil {
		a.selectionCancel()
	}
	if a.cancel != nil {
		a.cancel()
	}
	a.mu.Unlock()
	a.wg.Wait()
}
func main() {
	app := &App{}
	err := wails.Run(&options.App{Title: "GlobalSpeed", Width: 520, Height: 760, MinWidth: 480, MinHeight: 680,
		AssetServer: &assetserver.Options{Assets: frontend.Assets}, OnStartup: app.startup, OnShutdown: app.shutdown, Bind: []interface{}{app},
		Linux:   &linux.Options{Icon: appIcon, ProgramName: "globalspeed-desktop", WebviewGpuPolicy: linux.WebviewGpuPolicyNever},
		Windows: &windows.Options{BackdropType: windows.Mica, WebviewIsTransparent: true, WindowIsTranslucent: true},
	})
	if err != nil {
		fmt.Println("桌面程序启动失败:", err)
	}
}

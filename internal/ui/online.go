package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"

	"github.com/brunoga/armrest/internal/cache"
	"github.com/brunoga/armrest/internal/i18n"
	"github.com/brunoga/armrest/internal/workspace"
	"github.com/brunoga/sofabaton/irsource"
)

// indexAge is how long an online source's index is kept before it is
// fetched again: they change slowly.
const indexAge = 7 * 24 * time.Hour

// online is the online sources' indexes, searched when adding a device.
type online struct {
	entries []irsource.Entry
	// loaded is set once every source's index was tried; err is why one
	// failed.
	loaded, loading bool
	err             error
	// waiting are called once loaded.
	waiting []func()
}

// loadOnline loads the online sources' indexes, from the cache when
// fresh, calling done once loaded.
func (a *App) loadOnline(done func()) {
	o := &a.online
	if o.loaded {
		done()
		return
	}
	o.waiting = append(o.waiting, done)
	if o.loading {
		return
	}
	o.loading = true
	a.async(func() {
		var all []irsource.Entry
		var errs []error
		for _, s := range irsource.Sources {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			e, err := a.irIndex(ctx, s)
			cancel()
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", s, err))
			}
			all = append(all, e...)
		}
		fyne.Do(func() {
			o.entries, o.err, o.loaded, o.loading = all, errors.Join(errs...), true, false
			for _, f := range o.waiting {
				f()
			}
			o.waiting = nil
		})
	})
}

// cachedIndex is a source's index: the cached one while fresh, else
// fetched (and cached), else the cached one however old.
func (a *App) cachedIndex(ctx context.Context, s irsource.Source) ([]irsource.Entry, error) {
	var old *cache.Index
	if a.cache != nil {
		old = a.cache.LoadIndex(s)
		if old != nil && a.now().Sub(old.Saved) < indexAge {
			return old.Entries, nil
		}
	}
	entries, err := irsource.Index(ctx, nil, s)
	if err != nil {
		if old != nil {
			return old.Entries, nil
		}
		return nil, err
	}
	if a.cache != nil {
		if err := a.cache.SaveIndex(s, cache.Index{Saved: a.now(), Entries: entries}); err != nil {
			fyne.LogError("caching an index", err)
		}
	}
	return entries, nil
}

// importOnline fetches an online source's file of a device's codes and
// adds the device once the user confirms.
func (a *App) importOnline(e irsource.Entry, fetching func(bool)) {
	fetching(true)
	a.async(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		body, err := a.irFetch(ctx, e)
		var im *workspace.Imported
		if err == nil {
			im, err = workspace.FromIRFile(e.File(), strings.NewReader(string(body)))
		}
		fyne.Do(func() {
			fetching(false)
			if err != nil {
				dialog.ShowError(err, a.win)
				return
			}
			im.Device.Name = e.Name()
			a.confirmImport(i18n.T("Add from %s", string(e.Source)), im)
		})
	})
}

// onlineAbout says where an online result comes from: its kind, its
// model for irdb (its files are a protocol's device and subdevice), and
// its source.
func onlineAbout(e irsource.Entry) string {
	parts := []string{e.Kind}
	if e.Source == irsource.IRDB {
		parts = append(parts, e.Model)
	}
	return strings.Join(append(parts, string(e.Source)), " · ")
}

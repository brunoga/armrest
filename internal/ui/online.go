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
	"github.com/brunoga/sofabaton/profile"
)

// indexAge is how long an online source's index is kept before it is
// fetched again: they change slowly.
const indexAge = 7 * 24 * time.Hour

// online is the online sources' indexes, searched when adding a device.
type online struct {
	entries []irsource.Entry
	// brands are Sofabaton's catalog's makers, when it is on; models
	// their models, fetched when a search names the maker, by its id
	// (fetching while models[id] is nil but present).
	brands []irsource.SofabatonBrand
	models map[string][]irsource.Entry
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
	var on []irsource.Source
	for _, s := range sources {
		if sourceOn(s) {
			on = append(on, s)
		}
	}
	a.async(func() {
		var all []irsource.Entry
		var brands []irsource.SofabatonBrand
		var errs []error
		for _, s := range on {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			e, err := a.irIndex(ctx, s)
			cancel()
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", s, err))
			}
			if s == irsource.Sofabaton {
				// Its index is its makers: their models come when named.
				for _, b := range e {
					brands = append(brands, irsource.SofabatonBrand{ID: b.Path, Name: b.Brand})
				}
				continue
			}
			all = append(all, e...)
		}
		fyne.Do(func() {
			o.entries, o.err, o.loaded, o.loading = all, errors.Join(errs...), true, false
			o.brands, o.models = brands, map[string][]irsource.Entry{}
			for _, f := range o.waiting {
				f()
			}
			o.waiting = nil
		})
	})
}

// cachedIndex is a source's index: the cached one while fresh, else
// fetched (and cached), else the cached one however old. Sofabaton's is
// its makers, as entries of no model, their ids as paths.
func (a *App) cachedIndex(ctx context.Context, s irsource.Source) ([]irsource.Entry, error) {
	return a.cached(ctx, s, func(ctx context.Context) ([]irsource.Entry, error) {
		if s != irsource.Sofabaton {
			return irsource.Index(ctx, nil, s)
		}
		brands, err := a.sofabaton(sofabatonKey()).Brands(ctx)
		var out []irsource.Entry
		for _, b := range brands {
			out = append(out, irsource.Entry{Source: s, Brand: b.Name, Path: b.ID})
		}
		return out, err
	})
}

// cachedModels is a Sofabaton maker's models, cached as its index is.
func (a *App) cachedModels(ctx context.Context, b irsource.SofabatonBrand) ([]irsource.Entry, error) {
	return a.cached(ctx, irsource.Sofabaton+irsource.Source("-"+b.ID), func(ctx context.Context) ([]irsource.Entry, error) {
		return a.sofabaton(sofabatonKey()).Models(ctx, b)
	})
}

// cached is what fetch gets, kept as the index named s.
func (a *App) cached(ctx context.Context, s irsource.Source, fetch func(context.Context) ([]irsource.Entry, error)) ([]irsource.Entry, error) {
	var old *cache.Index
	if a.cache != nil {
		old = a.cache.LoadIndex(s)
		if old != nil && a.now().Sub(old.Saved) < indexAge {
			return old.Entries, nil
		}
	}
	entries, err := fetch(ctx)
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
			if c, ok := sofabatonCategories[strings.ToLower(e.Kind)]; ok && e.Source == irsource.Sofabaton {
				im.Device.Category = c
			}
			a.confirmImport(i18n.T("Add from %s", string(e.Source)), im)
		})
	})
}

// sofabatonCategories are the categories of Sofabaton's kinds of device:
// its catalog's type ids, which the app writes in the record.
var sofabatonCategories = map[string]profile.Category{
	"set top box": 1, "tv": 2, "stream box": 3, "dvd&blu-ray": 4, "projector": 6, "audio": 7,
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

// sofabatonNamed are the Sofabaton makers a search names: the maker
// picked, or one whose name is a word of the query, or two words of it.
func (o *online) sofabatonNamed(query, maker string) []irsource.SofabatonBrand {
	if len(o.brands) == 0 {
		return nil
	}
	key := func(s string) string {
		return strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, strings.ToLower(s))
	}
	want := map[string]bool{}
	if maker != "" {
		want[key(maker)] = true
	}
	words := strings.Fields(query)
	for i, w := range words {
		want[key(w)] = true
		if i+1 < len(words) {
			want[key(w+words[i+1])] = true
		}
	}
	delete(want, "")
	var out []irsource.SofabatonBrand
	for _, b := range o.brands {
		if want[key(b.Name)] {
			out = append(out, b)
		}
	}
	return out
}

// sofabatonModels are the models of the makers a search names that are
// fetched; the others are fetched, calling done once each is.
func (a *App) sofabatonModels(query, maker string, done func(irsource.SofabatonBrand, error)) []irsource.Entry {
	o := &a.online
	named := o.sofabatonNamed(query, maker)
	for _, b := range named {
		if _, asked := o.models[b.ID]; asked {
			continue
		}
		o.models[b.ID] = nil
		a.async(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			models, err := a.cachedModels(ctx, b)
			fyne.Do(func() {
				if err != nil {
					delete(o.models, b.ID) // asked again by the next search
				} else {
					o.models[b.ID] = models
				}
				done(b, err)
			})
		})
	}
	var out []irsource.Entry
	for _, b := range named {
		out = append(out, o.models[b.ID]...)
	}
	return out
}

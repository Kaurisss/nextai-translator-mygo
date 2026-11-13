package main

import (
	"log"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/Kaurisss/nextai-translator-mygo/internal/app"
	"github.com/Kaurisss/nextai-translator-mygo/internal/core"
	"github.com/Kaurisss/nextai-translator-mygo/resources"
)

func main() {
	mygo.App.SetName("NextAI Translator Native")
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	dataDir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		log.Fatal(err)
	}
	store := core.Store{Path: filepath.Join(dataDir, "data.json")}
	data, err := store.Load()
	if err != nil {
		log.Fatal(err)
	}
	a := app.New(data, store)
	if data.Settings.UILanguage != "" {
		mygo.App.SetLocale(data.Settings.UILanguage)
	}
	mygo.App.SetMenu(a.Menu())
	mygo.App.WhenReady(func() { a.Open(resources.Icon) })
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

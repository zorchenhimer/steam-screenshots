package steamscreenshots

import (
	"encoding/json"
	"sync"
	"os"
	"errors"
)

// GameIDs maps appids to display names
// var Games *GameList
type GameIDs map[string]string

type GameList struct {
	games    GameIDs
	m        sync.Mutex
	filename string
}

func LoadGameList(filename string) (*GameList, error) {
	gl := &GameList{
		games: make(map[string]string),
		m: sync.Mutex{},
		filename: filename,
	}

	file, err := os.Open(filename)
	if errors.Is(err, os.ErrNotExist) {
		return gl, nil
	} else if err != nil {
		return nil, err
	}
	defer file.Close()

	var games GameIDs
	dec := json.NewDecoder(file)
	err = dec.Decode(&games)
	if err != nil {
		return nil, err
	}

	gl.games = games

	return gl, nil
}

func ParseGames(raw []byte) (*GameList, error) {
	games := make(map[string]string)

	err := json.Unmarshal(raw, &games)
	if err != nil {
		return nil, err
	}

	return &GameList{games: games}, nil
}

func (g *GameList) Get(id string) string {
	g.m.Lock()
	defer g.m.Unlock()

	if val, ok := g.games[id]; ok {
		return val
	}
	return id
}

func (g *GameList) Set(id, val string) string {
	g.m.Lock()
	defer g.m.Unlock()

	g.games[id] = val
	return val
}

func (g *GameList) Update(list GameIDs) {
	g.m.Lock()
	defer g.m.Unlock()

	for key, val := range list {
		g.games[key] = val
	}
}

func (g *GameList) GetMap() GameIDs {
	g.m.Lock()
	defer g.m.Unlock()

	retList := GameIDs{}
	for key, val := range g.games {
		retList[key] = val
	}
	return retList
}

func (g *GameList) Length() int {
	g.m.Lock()
	defer g.m.Unlock()

	return len(g.games)
}

package steamscreenshots

import (
	"sync"
)

// GameIDs maps appids to display names
//var Games *GameList
type GameIDs map[string]string

type GameList struct {
	games GameIDs
	m     sync.Mutex
}

func NewGameList() *GameList {
	return &GameList{
		games: make(map[string]string),
	}
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

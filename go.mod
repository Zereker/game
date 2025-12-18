module github.com/Zereker/game

go 1.23

require (
	github.com/Zereker/socket v0.0.0
	github.com/Zereker/werewolf v0.0.0
	github.com/google/uuid v1.6.0
	github.com/pkg/errors v0.9.1
)

require golang.org/x/sync v0.0.0-20190423024810-112230192c58 // indirect

replace (
	github.com/Zereker/socket => ../socket
	github.com/Zereker/werewolf => ../werewolf
)

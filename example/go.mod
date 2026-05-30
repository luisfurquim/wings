module github.com/luisfurquim/wings/example

go 1.25.0

require (
	github.com/luisfurquim/goose v0.1.0
	github.com/luisfurquim/wings v0.0.0
)

replace (
	github.com/luisfurquim/goose => ../../goose
	github.com/luisfurquim/wings => ../
)

module github.com/flashbacks/ocr

go 1.25.0

require (
	github.com/anthonynsimon/bild v0.14.0
	github.com/disintegration/imaging v1.6.2
	github.com/flashbacks/shared v0.0.0-00010101000000-000000000000
	github.com/otiai10/gosseract/v2 v2.4.1
)

require golang.org/x/image v0.18.0 // indirect

replace github.com/flashbacks/shared => ../shared

package main

// Cache image dimensions in a file

import (
    "bufio"
    "path/filepath"
    "fmt"
    "errors"
    "image/jpeg"
    //"image/png"
    "io"
    "os"
    "strconv"
    "strings"
)

var ImageCache *ImageData
type ImageDimensions struct {
     width  int
     height int
}

func (i ImageDimensions) String() string {
    return fmt.Sprintf("(%d, %d)", i.width, i.height)
}

type ImageData struct {
    current int
// TODO: AAAAAAAAAAAHHHHHHHHHHHHH
    keys []string
    vals []*ImageDimensions
}

var NoMoreImages error = errors.New("No more images")
var MismatchError error = errors.New("Mismatched key/val lengths")
var NotImplementedError error = errors.New("Not implemented")

func NewImageData() (*ImageData) {
    return &ImageData{
        current:    0,
        keys:       []string{},
        vals:       []*ImageDimensions{},
    }
}

func (i *ImageData) Merge(n *ImageData) error {
    if len(i.keys) != len(i.vals) {
        fmt.Println("Original image data mismatch")
        return MismatchError
    }

    if len(n.keys) != len(n.vals) {
        fmt.Println("New image data mismatch")
        return MismatchError
    }

    maxlen := n.Length()
    for x := 0; x < maxlen; x++ {
        p, d, err := n.GetByIndex(x)
        if err != nil {
            return fmt.Errorf("Image merge error: %s", err)
        }
        i.Add(p, d)
    }
    return nil
}

func (i *ImageData) GetByPath(path string) (*ImageDimensions, error) {
    if len(i.keys) != len(i.vals) {
        return nil, MismatchError
    }

    for x := 0; x < len(i.keys); x++ {
        if i.keys[x] == path {
            return i.vals[x], nil
        }
    }

    return nil, fmt.Errorf("Image data not found")
}

func (i *ImageData) GetByIndex(idx int) (string, *ImageDimensions, error) {
    if len(i.keys) != len(i.vals) {
        return "", nil, MismatchError
    }

    if idx >= len(i.keys) {
        return "", nil, fmt.Errorf("Index out of range")
    }

    return i.keys[idx], i.vals[idx], nil
}

func (i ImageData) Length() int {
    return len(i.keys)
}

func (i *ImageData) Add(path string, dims *ImageDimensions) error {
    if len(i.keys) != len(i.vals) {
        return MismatchError
    }

    i.keys = append(i.keys, path)
    i.vals = append(i.vals, dims)
    return nil
}

func (i *ImageData) Remove(path string) (string, *ImageDimensions, error) {
    if len(i.keys) != len(i.vals) {
        return "", nil, MismatchError
    }

    var remPath string
    var remDims *ImageDimensions

    newKeys := []string{}
    newVals := []*ImageDimensions{}

    for x := 0; x < len(i.keys); x++ {
        if path == i.keys[x] {
            remPath = i.keys[x]
            remDims = i.vals[x]
        } else {

            newKeys = append(newKeys, i.keys[x])
            newVals = append(newVals, i.vals[x])
        }
    }

    i.keys = newKeys
    i.vals = newVals
    return remPath, remDims, nil
}

func (i *ImageData) Next() (string, *ImageDimensions, error) {
    if len(i.keys) != len(i.vals) {
        return "", nil, MismatchError
    }

    if i.current >= len(i.keys) {
        return "", nil, NoMoreImages
    }

    defer func() { i.current++ }()
    return i.keys[i.current], i.vals[i.current], nil
}

func LoadCachedImageData(filename string) (*ImageData, error) {
    file, err := os.Open(filename)
    if err != nil {
        return nil, fmt.Errorf("Unable to load image data file: %s", err)
    }
    defer file.Close()

    data := NewImageData()
    reader := bufio.NewReader(file)
    line, err := reader.ReadString('\n')
    for err == nil {
        // Skip blank lines
        line = strings.TrimSpace(line)
        if len(line) == 0 {
            continue
        }

        // Skip malformed lines
        var path string
        var dims *ImageDimensions
        path, dims, err = parseLine(line)
        if err != nil {
            continue
        }

        // TODO: Validate path?
        if err := data.Add(path, dims); err != nil {
            // Fail here, could be something else wrong.
            return nil, fmt.Errorf("Unable to add image dimensions: %s", err)
        }

        line, err = reader.ReadString('\n')
    }

    if err != io.EOF {
        return nil, fmt.Errorf("Unable to read image data file: %s", err)
    }

    //retData := ImageData(data)
    return data, nil
}

func ParseImages(path string) (*ImageData, error) {
    dir, err := filepath.Glob(filepath.Join(path, "*.jpg"))
    if err != nil {
        return nil, err
    }

    data := NewImageData()
    for _, f := range dir {
        dims, err := readImage(filepath.Join(path, f))
        if err != nil {
            fmt.Println(err)
            continue
        }

        if err := data.Add(f, dims); err != nil {
            fmt.Println(err)
        }
    }

    return data, nil
}

func readImage(fullpath string) (*ImageDimensions, error) {
    //fmt.Printf("reading image %q\n", fullpath)
    file, err := os.Open(fullpath)
    if err != nil {
        return nil, fmt.Errorf("Unable to open %q: %s", fullpath, err)
    }
    defer file.Close()

    img, err := jpeg.Decode(file)
    if err != nil {
        return nil, fmt.Errorf("Unable to decode %q: %s", fullpath, err)
    }
    pt := img.Bounds().Max

    return &ImageDimensions{ width: pt.X, height: pt.Y }, nil
}

// TODO: Add option to ignore malformed data lines?
func parseLine(line string) (string, *ImageDimensions, error) {
    items := strings.Split(line, "\t")
    if len(items) != 3 {
        return "", nil, fmt.Errorf("Malformed data line: %q", line)
    }

    w, err := strconv.ParseUint(items[1], 10, 32)
    if err != nil {
        return "", nil, fmt.Errorf("Invalid width %q from line: %q", items[1], line)
    }

    h, err := strconv.ParseUint(items[2], 10, 32)
    if err != nil {
        return "", nil, fmt.Errorf("Invalid height %q from line: %q", items[1], line)
    }

    return items[0], &ImageDimensions{width: int(w), height: int(h)}, nil
}

func (i *ImageData) Save(filename string) (error) {
    file, err := os.Create(filename)
    if err != nil {
        return fmt.Errorf("Unable to save image data: %s", err)
    }
    defer file.Close()

    for x := 0; x < len(i.keys); x++ {
        _, err := fmt.Fprintf(file, "%s\t%d\t%d\n", i.keys[x], i.vals[x].width, i.vals[x].height)
        if err != nil {
            return err
        }
    }

    return nil
}

func (i *ImageData) Dump() {
    fmt.Println("Duming ImageCache")
    if len(i.keys) != len(i.vals) {
        fmt.Printf("Mismatch! i.keys: %d; i.vals: %d\n", len(i.keys), len(i.vals))
    }

    for x := 0; x < len(i.keys) || x < len(i.vals); x++ {
        keystr := "<NONE>"
        valstr := "<NONE>"

        if x < len(i.keys) {
            keystr = i.keys[x]
        }
        if x < len(i.vals) {
            valstr = i.vals[x].String()
        }

        fmt.Printf("%s: %s\n", keystr, valstr)
    }
}

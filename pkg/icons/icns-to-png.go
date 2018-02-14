package icons

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/apex/log"
	"github.com/develar/app-builder/pkg/fs"
	"github.com/develar/app-builder/pkg/util"
	"github.com/develar/errors"
	"github.com/disintegration/imaging"
)

type Icns2PngMapping struct {
	Id   string
	Size int
}

var icnsTypeToSize = []Icns2PngMapping{
	{"is32", 16},
	{"il32", 32},
	{"ih32", 48},
	{"icp6", 64},
	{"it32", 128},
	{ICNS_256, 256},
	{ICNS_512, 512},
}

func ConvertIcnsToPng(inFile string) ([]IconInfo, error) {
	tempDir, err := util.TempDir("", ".iconset")
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var maxIconPath string
	var result []IconInfo

	sizeList := []int{24, 96}
	outFileTemplate := filepath.Join(tempDir, "icon_%dx%d.png")
	maxSize := 0
	if runtime.GOOS == "darwin" && os.Getenv("FORCE_ICNS2PNG") == "" {
		output, err := exec.Command("iconutil", "--convert", "iconset", "--output", tempDir, inFile).CombinedOutput()
		if err != nil {
			log.Info(string(output))
			return nil, errors.WithStack(err)
		}

		iconFileNames, err := fs.ReadDirContent(tempDir)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		for _, item := range icnsTypeToSize {
			fileName := fmt.Sprintf("icon_%dx%d.png", item.Size, item.Size)
			if contains(iconFileNames, fileName) {
				// list sorted by size, so, last assignment is a max size
				maxIconPath = filepath.Join(tempDir, fileName)
				maxSize = item.Size
				result = append(result, IconInfo{maxIconPath, item.Size})
			} else {
				sizeList = append(sizeList, item.Size)
			}
		}
	} else {
		result, err = ConvertIcnsToPngUsingOpenJpeg(inFile, tempDir)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		sortBySize(result)
		for _, item := range icnsTypeToSize {
			if !hasSize(result, item.Size) {
				sizeList = append(sizeList, item.Size)
			}
		}

		maxIconInfo := result[len(result)-1]
		maxIconPath = maxIconInfo.File
		maxSize = maxIconInfo.Size
	}

	err = multiResizeImage(maxIconPath, outFileTemplate, &result, sizeList, maxSize)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	sortBySize(result)
	return result, nil
}

func hasSize(list []IconInfo, size int) bool {
	for _, info := range list {
		if info.Size == size {
			return true
		}
	}
	return false
}

func sortBySize(list []IconInfo) {
	sort.Slice(list, func(i, j int) bool { return list[i].Size < list[j].Size })
	return
}

func contains(files []string, name string) bool {
	for _, fileName := range files {
		if fileName == name {
			return true
		}
	}
	return false
}

func multiResizeImage(inFile string, outFileNameFormat string, result *[]IconInfo, sizeList []int, maxSize int) (error) {
	imageCount := len(sizeList)
	if imageCount == 0 {
		return nil
	}

	originalImage, err := LoadImage(inFile)
	if err != nil {
		return errors.WithStack(err)
	}

	return util.MapAsync(imageCount, func(taskIndex int) (func() error, error) {
		size := sizeList[taskIndex]
		if size > maxSize {
			return nil, nil
		}

		outFilePath := fmt.Sprintf(outFileNameFormat, size, size)
		*result = append(*result, IconInfo{
			File: outFilePath,
			Size: size,
		})

		return func() error {
			newImage := imaging.Resize(originalImage, size, size, imaging.Lanczos)
			return SaveImage(newImage, outFilePath)
		}, nil
	})
}
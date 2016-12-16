package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"os"
	"runtime"

	"github.com/Sirupsen/logrus"
	"github.com/containers/image/storage"
	"github.com/containers/image/transports"
	"github.com/containers/image/types"
	"github.com/containers/storage/pkg/reexec"
	sstorage "github.com/containers/storage/storage"
	"github.com/docker/distribution/digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	"github.com/opencontainers/image-spec/specs-go/v1"
)

var (
	imageName     = "kubernetes/pause"
	imageBinary   = "/pause"
	sourceBinary  = "../../pause/pause"
	rootDir       = ""
	runrootDir    = ""
	debug         = false
	storageDriver = ""
)

func main() {
	var store sstorage.Store

	if reexec.Init() {
		return
	}

	flags := flag.NewFlagSet("bin2img", flag.ContinueOnError)
	flags.BoolVar(&debug, "debug", debug, "turn on debug logging")
	flags.StringVar(&rootDir, "root", rootDir, "graph root directory")
	flags.StringVar(&runrootDir, "runroot", runrootDir, "run root directory")
	flags.StringVar(&storageDriver, "storage-driver", storageDriver, "storage driver")
	flags.StringVar(&imageName, "image-name", imageName, "set image name")
	flags.StringVar(&sourceBinary, "source-binary", sourceBinary, "source binary")
	flags.StringVar(&imageBinary, "image-binary", imageBinary, "image binary")
	if err := flags.Parse(os.Args[1:]); err != nil {
		status := 0
		if err != flag.ErrHelp {
			logrus.Errorf("error parsing command line arguments: %v", err)
			status = 1
		}
		os.Exit(status)
	}
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.ErrorLevel)
	}
	if rootDir == "" && runrootDir != "" {
		logrus.Errorf("must set --root and --runroot, or neither")
		os.Exit(1)
	}
	if rootDir != "" && runrootDir == "" {
		logrus.Errorf("must set --root and --runroot, or neither")
		os.Exit(1)
	}
	storeOptions := sstorage.DefaultStoreOptions
	if rootDir != "" && runrootDir != "" {
		storeOptions.GraphDriverName = storageDriver
		storeOptions.GraphRoot = rootDir
		storeOptions.RunRoot = runrootDir
	}
	store, err := sstorage.GetStore(storeOptions)
	if err != nil {
		logrus.Errorf("error opening storage: %v", err)
		os.Exit(1)
	}

	layerBuffer := &bytes.Buffer{}
	binary, err := os.Open(sourceBinary)
	if err != nil {
		logrus.Errorf("error opening image binary: %v", err)
		os.Exit(1)
	}
	binInfo, err := binary.Stat()
	if err != nil {
		logrus.Errorf("error statting image binary: %v", err)
		os.Exit(1)
	}
	archive := tar.NewWriter(layerBuffer)
	err = archive.WriteHeader(&tar.Header{
		Name:     imageBinary,
		Size:     binInfo.Size(),
		Mode:     0555,
		ModTime:  binInfo.ModTime(),
		Typeflag: tar.TypeReg,
		Uname:    "root",
		Gname:    "root",
	})
	if err != nil {
		logrus.Errorf("error writing archive header: %v", err)
		os.Exit(1)
	}
	_, err = io.Copy(archive, binary)
	if err != nil {
		logrus.Errorf("error archiving image binary: %v", err)
		os.Exit(1)
	}
	archive.Close()
	binary.Close()
	layerInfo := types.BlobInfo{
		Digest: digest.Canonical.FromBytes(layerBuffer.Bytes()),
		Size:   int64(layerBuffer.Len()),
	}

	storage.Transport.SetStore(store)
	imgName := storage.Transport.Name() + ":" + imageName
	ref, err := transports.ParseImageName(imgName)
	if err != nil {
		logrus.Errorf("error parsing image name: %v", err)
		os.Exit(1)
	}
	img, err := ref.NewImageDestination(nil)
	if err != nil {
		logrus.Errorf("error preparing to write image: %v", err)
		os.Exit(1)
	}
	layer, err := img.PutBlob(layerBuffer, layerInfo)
	if err != nil {
		logrus.Errorf("error preparing to write image: %v", err)
		os.Exit(1)
	}
	config := &v1.Image{
		Architecture: runtime.GOARCH,
		OS:           runtime.GOOS,
		Config: v1.ImageConfig{
			User:       "root",
			Entrypoint: []string{imageBinary},
		},
		RootFS: v1.RootFS{
			Type: "layers",
			DiffIDs: []string{
				layer.Digest.String(),
			},
		},
	}
	cbytes, err := json.Marshal(config)
	if err != nil {
		logrus.Errorf("error encoding configuration: %v", err)
		os.Exit(1)
	}
	configInfo := types.BlobInfo{
		Digest: digest.Canonical.FromBytes(cbytes),
		Size:   int64(len(cbytes)),
	}
	configInfo, err = img.PutBlob(bytes.NewBuffer(cbytes), configInfo)
	if err != nil {
		logrus.Errorf("error saving configuration: %v", err)
		os.Exit(1)
	}
	manifest := &v1.Manifest{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
			MediaType:     v1.MediaTypeImageManifest,
		},
		Config: v1.Descriptor{
			MediaType: v1.MediaTypeImageConfig,
			Digest:    configInfo.Digest.String(),
			Size:      int64(len(cbytes)),
		},
		Layers: []v1.Descriptor{{
			MediaType: v1.MediaTypeImageLayer,
			Digest:    layer.Digest.String(),
			Size:      layer.Size,
		}},
	}
	mbytes, err := json.Marshal(manifest)
	if err != nil {
		logrus.Errorf("error encoding manifest: %v", err)
		os.Exit(1)
	}
	err = img.PutManifest(mbytes)
	if err != nil {
		logrus.Errorf("error saving manifest: %v", err)
		os.Exit(1)
	}
	err = img.Commit()
	if err != nil {
		logrus.Errorf("error committing image: %v", err)
		os.Exit(1)
	}
}

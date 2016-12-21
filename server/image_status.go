package server

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/containers/storage/storage"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

// ImageStatus returns the status of the image.
func (s *Server) ImageStatus(ctx context.Context, req *pb.ImageStatusRequest) (*pb.ImageStatusResponse, error) {
	logrus.Debugf("ImageStatus: %+v", req)
	image := ""
	img := req.GetImage()
	if img != nil {
		image = img.GetImage()
	}
	if image == "" {
		return nil, fmt.Errorf("no image specified")
	}
	status, err := s.images.ImageStatus(s.imageContext, image)
	if err != nil {
		if err == storage.ErrImageUnknown {
			return &pb.ImageStatusResponse{}, nil
		}
		return nil, err
	}
	return &pb.ImageStatusResponse{
		Image: &pb.Image{
			Id:       &status.ID,
			RepoTags: status.Names,
			Size_:    status.Size,
		},
	}, nil
}

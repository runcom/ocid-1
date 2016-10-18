package server

import (
	"github.com/Sirupsen/logrus"
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
	status, err := s.images.ImageStatus(ctx, image)
	if err != nil {
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

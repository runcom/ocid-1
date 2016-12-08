package server

import (
	"github.com/Sirupsen/logrus"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

// RemoveImage removes the image.
func (s *Server) RemoveImage(ctx context.Context, req *pb.RemoveImageRequest) (*pb.RemoveImageResponse, error) {
	logrus.Debugf("RemoveImage: %+v", req)
	image := ""
	img := req.GetImage()
	if img != nil {
		image = img.GetImage()
	}
	err := s.images.RemoveImage(ctx, image)
	if err != nil {
		return nil, err
	}
	return &pb.RemoveImageResponse{}, nil
}

package server

import (
	"github.com/Sirupsen/logrus"
	"github.com/containers/image/copy"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

// PullImage pulls a image with authentication config.
func (s *Server) PullImage(ctx context.Context, req *pb.PullImageRequest) (*pb.PullImageResponse, error) {
	logrus.Debugf("PullImage: %+v", req)
	// TODO(runcom?): deal with AuthConfig in req.GetAuth()
	// TODO(somebody?): either rework PullImage to verify signatures, or do it by using PullImageUsingContexts, if that's enough
	// TODO: what else do we need here? (Signatures when the story isn't just pulling from docker://)
	image := ""
	img := req.GetImage()
	if img != nil {
		image = img.GetImage()
	}
	options := &copy.Options{}
	_, err := s.images.PullImage(s.imageContext, image, options)
	if err != nil {
		return nil, err
	}
	return &pb.PullImageResponse{}, nil
}

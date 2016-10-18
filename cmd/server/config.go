package main

import (
	"os"
	"text/template"

	"github.com/kubernetes-incubator/cri-o/server"
	"github.com/opencontainers/runc/libcontainer/selinux"
	"github.com/urfave/cli"
)

const (
	ocidRoot            = "/var/lib/containers"
	ocidRunRoot         = "/var/run/containers"
	conmonPath          = "/usr/libexec/ocid/conmon"
	pauseImage          = "kubernetes/pause"
	pauseCommand        = "/pause"
	defaultTransport    = "docker://"
	seccompProfilePath  = "/etc/ocid/seccomp.json"
	apparmorProfileName = "ocid-default"
	cgroupManager       = "cgroupfs"
	cniConfigDir        = "/etc/cni/net.d/"
	cniBinDir           = "/opt/cni/bin/"
)

var commentedConfigTemplate = template.Must(template.New("config").Parse(`
# The "ocid" table contains all of the server options.
[ocid]

# root is a path to the "root directory". OCID stores all of its data,
# including container images, in this directory.
root = "{{ .Root }}"

# run is a path to the "run directory". OCID stores all of its state
# in this directory.
runroot = "{{ .RunRoot }}"

# storage-driver select which storage driver is used to manage storage
# of images and containers.
storage-driver = "{{ .Storage }}"

# storage-option is used to pass an option to the storage driver.
storage-option = [
{{ range $opt := .StorageOptions }}{{ printf "\t%q,\n" $opt }}{{ end }}]

# The "ocid.api" table contains settings for the kubelet/gRPC
# interface (which is also used by ocic).
[ocid.api]

# listen is the path to the AF_LOCAL socket on which ocid will listen.
listen = "{{ .Listen }}"

# The "ocid.runtime" table contains settings pertaining to the OCI
# runtime used and options for how to set up and manage the OCI runtime.
[ocid.runtime]

# runtime is a path to the OCI runtime which ocid will be using.
runtime = "{{ .Runtime }}"

# conmon is the path to conmon binary, used for managing the runtime.
conmon = "{{ .Conmon }}"

# conmon_env is the environment variable list for conmon process,
# used for passing necessary environment variable to conmon or runtime.
conmon_env = [
{{ range $env := .ConmonEnv }}{{ printf "\t%q,\n" $env }}{{ end }}]

# selinux indicates whether or not SELinux will be used for pod
# separation on the host. If you enable this flag, SELinux must be running
# on the host.
selinux = {{ .SELinux }}

# seccomp_profile is the seccomp json profile path which is used as the
# default for the runtime.
seccomp_profile = "{{ .SeccompProfile }}"

# apparmor_profile is the apparmor profile name which is used as the
# default for the runtime.
apparmor_profile = "{{ .ApparmorProfile }}"

# cgroup_manager is the cgroup management implementation to be used
# for the runtime.
cgroup_manager = "{{ .CgroupManager }}"

# The "ocid.image" table contains settings pertaining to the
# management of OCI images.
[ocid.image]

# default-transport is the prefix we try prepending to an image name if the
# image name as we receive it can't be parsed as a valid source reference
default-transport = "{{ .DefaultTransport }}"

# pause-image is the image which we use to instantiate infra containers.
pause-image = "{{ .PauseImage }}"

# pause-command is the command to run in a pause-image to have a container just
# sit there.  If the image contains the necessary information, this value need
# not be specified.
pause-command = "{{ .PauseCommand }}"

# The "ocid.network" table contains settings pertaining to the
# management of CNI plugins.
[ocid.network]

# network_dir is is where CNI network configuration
# files are stored.
network_dir = "{{ .NetworkDir }}"

# plugin_dir is is where CNI plugin binaries are stored.
plugin_dir = "{{ .PluginDir }}"
`))

// TODO: Currently ImageDir isn't really used, so we haven't added it to this
//       template. Add it once the storage code has been merged.

// DefaultConfig returns the default configuration for ocid.
func DefaultConfig() *server.Config {
	return &server.Config{
		RootConfig: server.RootConfig{
			Root:    ocidRoot,
			RunRoot: ocidRunRoot,
			LogDir:  "/var/log/ocid/pods",
		},
		APIConfig: server.APIConfig{
			Listen: "/var/run/ocid.sock",
		},
		RuntimeConfig: server.RuntimeConfig{
			Runtime: "/usr/bin/runc",
			Conmon:  conmonPath,
			ConmonEnv: []string{
				"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			},
			SELinux:         selinux.SelinuxEnabled(),
			SeccompProfile:  seccompProfilePath,
			ApparmorProfile: apparmorProfileName,
			CgroupManager:   cgroupManager,
		},
		ImageConfig: server.ImageConfig{
			DefaultTransport: defaultTransport,
			PauseImage:       pauseImage,
			PauseCommand:     pauseCommand,
		},
		NetworkConfig: server.NetworkConfig{
			NetworkDir: cniConfigDir,
			PluginDir:  cniBinDir,
		},
	}
}

var configCommand = cli.Command{
	Name:  "config",
	Usage: "generate ocid configuration files",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "default",
			Usage: "output the default configuration",
		},
	},
	Action: func(c *cli.Context) error {
		// At this point, app.Before has already parsed the user's chosen
		// config file. So no need to handle that here.
		config := c.App.Metadata["config"].(*server.Config)
		if c.Bool("default") {
			config = DefaultConfig()
		}

		// Output the commented config.
		return commentedConfigTemplate.ExecuteTemplate(os.Stdout, "config", config)
	},
}

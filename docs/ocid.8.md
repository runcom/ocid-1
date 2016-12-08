% ocid(8) Open Container Initiative Daemon
% Dan Walsh
% SEPTEMBER 2016
# NAME
ocid - Enable OCI Kubernetes Container Runtime daemon

# SYNOPSIS
**ocid**
[**--config**=[*value*]]
[**--conmon**=[*value*]]
[**--debug**]
[**--default_transport**=[*value*]]
[**--help**|**-h**]
[**--listen**=[*value*]]
[**--log**=[*value*]]
[**--log-format value**]
[**--pause_command**=[*value*]]
[**--pause_image**=[*value*]]
[**--root**=[*value*]]
[**--runroot**=[*value*]]
[**--runtime**=[*value*]]
[**--storage_driver**=[*value*]]
[**--storage_option**=[*value*]]
[**--selinux**]
[**--seccomp-profile**=[*value*]]
[**--apparmor-profile**=[*value*]]
[**--version**|**-v**]

# DESCRIPTION
OCI-based implementation of Kubernetes Container Runtime Interface Daemon

ocid is meant to provide an integration path between OCI conformant runtimes and the kubelet. Specifically, it implements the Kubelet Container Runtime Interface (CRI) using OCI conformant runtimes. The scope of ocid is tied to the scope of the CRI.

	* Support multiple image formats including the existing Docker image format
	* Support for multiple means to download images including trust & image verification
	* Container image management (managing image layers, overlay filesystems, etc)
	* Container process lifecycle management
	* Monitoring and logging required to satisfy the CRI
	* Resource isolation as required by the CRI

**ocid [GLOBAL OPTIONS]**

**ocid [GLOBAL OPTIONS] config [OPTIONS]**

# GLOBAL OPTIONS

**--apparmor_profile**=""
  Name of the apparmor profile to be used as the runtime's default (default: "ocid-default")

**--config**=""
  path to configuration file

**--conmon**=""
  path to the conmon executable (default: "/usr/libexec/ocid/conmon")

**--debug**
  Enable debug output for logging

**--default_transport**
  A prefix to prepend to image names that can't be pulled as-is.

**--help, -h**
  Print usage statement

**--listen**=""
  Path to ocid socket (default: "/var/run/ocid.sock")

**--log**=""
  Set the log file path where internal debug information is written

**--log-format**=""
  Set the format used by logs ('text' (default), or 'json') (default: "text")

**--pause_command**=""
  Path to the pause executable in the pause image (default: "/pause")

**--pause_image**=""
  Image which contains the pause executable (default: "kubernetes/pause")

**--root**=""
  OCID root dir (default: "/var/lib/containers")

**--runroot**=""
  OCID state dir (default: "/var/run/containers")

**--runtime**=""
  OCI runtime path (default: "/usr/bin/runc")

**--selinux**=*true*|*false*
  Enable selinux support (default: false)

**--seccomp_profile**=""
  Path to the seccomp json profile to be used as the runtime's default (default: "/etc/ocid/seccomp.json")

**--storage_driver**
  OCI storage driver (default: "devicemapper")

**--storage_option**
  OCI storage driver option (no default)

**--version, -v**
  Print the version

# COMMANDS
OCID's default command is to start the daemon. However, it currently offers a
single additional subcommand.

## config

Outputs a commented version of the configuration file that would've been used
by OCID. This allows you to save you current configuration setup and then load
it later with **--config**. Global options will modify the output.

**--default**
  Output the default configuration (without taking into account any configuration options).

# SEE ALSO
ocid.conf(5)

# HISTORY
Sept 2016, Originally compiled by Dan Walsh <dwalsh@redhat.com> and Aleksa Sarai <asarai@suse.de>

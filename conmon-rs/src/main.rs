#[macro_use]
extern crate clap;

extern crate nix;
use std::env;

fn main() {
    let cwd = env::current_dir().unwrap();
    let options = clap::App::new("conmon-rs")
        .setting(clap::AppSettings::TrailingVarArg)
        .version(crate_version!())
        .about("shim between CRI-O and runc")
        .arg(clap::Arg::with_name("no-pivot")
            .long("no-pivot")
            .help("Do not use pivot_root")
            .takes_value(false))
        .arg(clap::Arg::with_name("terminal")
            .short("t")
            .long("terminal")
            .help("Set terminal")
            .takes_value(false))
        .arg(clap::Arg::with_name("stdin")
            .short("i")
            .long("stdin")
            .help("Set stdin")
            .takes_value(false))
        .arg(clap::Arg::with_name("systemd-cgroup")
            .short("s")
            .long("systemd-cgroup")
            .help("Enable systemd cgroup manager")
            .takes_value(false))
        .arg(clap::Arg::with_name("cid")
            .short("c")
            .long("cid")
            .help("Container ID")
            .required(true)
            .empty_values(false))
        .arg(clap::Arg::with_name("cuuid")
            .short("u")
            .long("cuuid")
            .help("Container UUID")
            .required_unless("exec")
            .empty_values(false))
        .arg(clap::Arg::with_name("exit-dir")
            .long("exit-dir")
            .help("Path to the directory where exit files are written")
            .required_unless("exec")
            .empty_values(false))
        .arg(clap::Arg::with_name("exec")
            .short("e")
            .long("exec")
            .requires("exec-process-spec")
            .help("Exec a command in a running container")
            .takes_value(false))
        .arg(clap::Arg::with_name("exec-process-spec")
            .long("exec-process-spec")
            .help("Path to the process spec for exec")
            .requires("exec")
            .empty_values(false))
        .arg(clap::Arg::with_name("runtime")
            .short("r")
            .long("runtime")
            .help("Runtime path")
            .required(true)
            .empty_values(false))
        .arg(clap::Arg::with_name("log-path")
            .short("l")
            .long("log-path")
            .help("Log file path")
            .required(true)
            .empty_values(false))
        .arg(clap::Arg::with_name("pid-file")
            .short("p")
            .long("pid-file")
            .takes_value(true)
            .help("PID file")
            .empty_values(false))
        .arg(clap::Arg::with_name("timeout")
            .short("T")
            .long("timeout")
            .takes_value(true)
            .empty_values(false)
            .help("Timeout for operations in seconds"))
        .arg(clap::Arg::with_name("log-size-max")
            .long("log-size-max")
            .takes_value(true)
            .empty_values(false)
            .help("Maximum size of log file in bytes"))
        .arg(clap::Arg::with_name("socket-dir-path")
            .long("socket-dir-path")
            .help("Location of container attach sockets")
            .default_value("/var/lib/crio")
            .empty_values(false))
        .arg(clap::Arg::with_name("bundle")
            .short("b")
            .long("bundle")
            .help("Bundle path")
            .required_unless("exec")
            .default_value(cwd.to_str().unwrap())
            .empty_values(false))
        .get_matches();

    let opt_timeout: u64 = match options.value_of("timeout") {
        Some(x) => x.parse().expect("not a number"),
        None => 0,
    };
    println!("{}", opt_timeout);
    let opt_log_size_max: i64 = match options.value_of("log-size-max") {
        Some(x) => x.parse().expect("not a number"),
        None => -1,
    };
    println!("{}", opt_log_size_max);

    let opt_terminal = options.is_present("terminal");
    println!("{}", opt_terminal);
    let opt_stdin = options.is_present("stdin");
    println!("{}", opt_stdin);

    let dev_null_r = nix::fcntl::open("/dev/null",
                                      nix::fcntl::O_RDONLY | nix::fcntl::O_CLOEXEC,
                                      nix::sys::stat::Mode::empty())
        .unwrap();
    let dev_null_w = nix::fcntl::open("/dev/null",
                                      nix::fcntl::O_WRONLY | nix::fcntl::O_CLOEXEC,
                                      nix::sys::stat::Mode::empty())
        .unwrap();
    println!("{:?}", dev_null_r);
    println!("{:?}", dev_null_w);

    let mut buf = vec![0; 8192];
    let start_pipe_fd = get_pipe_fd_from_env("_OCI_STARTPIPE");
    if start_pipe_fd >= 0 {
        // Block for an initial write to the start pipe before
        // spawning any childred or exiting, to ensure the
        // parent can put us in the right cgroup.
        // TODO(runcom): check return
        nix::unistd::read(start_pipe_fd, &mut buf);
        // TODO(runcom): check return
        nix::unistd::close(start_pipe_fd);
    }

    let opt_cid = options.value_of("cid").unwrap();
    let default_pidfile = format!("{}/pidfile-{}", cwd.to_str().unwrap(), opt_cid);
    let opt_pid_file = options.value_of("pid-file")
        .unwrap_or_else(|| default_pidfile.as_str());
    println!("{}", opt_pid_file)
}

fn get_pipe_fd_from_env(key: &str) -> std::os::unix::io::RawFd {
    match env::var_os(key) {
        Some(val) => {
            let pipe_fd = val.into_string().unwrap().parse().expect("not a number");
            match nix::fcntl::fcntl(pipe_fd as std::os::unix::io::RawFd,
                                    nix::fcntl::FcntlArg::F_SETFD(nix::fcntl::FD_CLOEXEC)) {
                Ok(_) => pipe_fd,
                // TODO(runcom): replace panic! with something better as it
                // currently returns exit code == 101 which is bad
                Err(e) => panic!("unable to make {} CLOEXEC: {}", pipe_fd, e),
            }
        }
        None => -1,
    }
}

# WireGuard Portal on Raspberry Pi

This readme only contains a detailed explanation of how to set up the WireGuard Portal service on a raspberry pi (>= 3).

## Setup

You can download prebuild binaries from the [release page](https://github.com/h44z/wg-portal/releases). If you want to build the binary yourself,
use the following instructions:

### Building
This section describes how to build the WireGuard Portal code.
To compile the final binary, use the Makefile provided in the repository.
As WireGuard Portal is written in Go, **golang >= 1.16** must be installed prior to building.

```
make build-cross-plat
```

The compiled binary and all necessary assets will be located in the dist folder.

### Service setup

 - Copy the contents from the dist folder (or from the downloaded zip file) to `/opt/wg-portal`. You can choose a different path as well, but make sure to update the systemd service file accordingly.
 - Update the provided systemd `wg-portal.service` file:
   - Make sure that the binary matches the system architecture. 
     - There are three pre-build binaries available: wg-portal-**amd64**, wg-portal-**arm64** and wg-portal-**arm**.
     - For a raspberry pi use the arm binary if you are using armv7l architecture. If armv8 is used, the arm64 version should work.
   - Make sure that the paths to the binary and the working directory are set correctly (defaults to /opt/wg-portal/wg-portal-amd64):
     - ConditionPathExists
     - WorkingDirectory
     - ExecStart
     - EnvironmentFile
   - Update environment variables in the `wg-portal.env` file to fit your needs
 - Make sure that the binary application file is executable
   - `sudo chmod +x /opt/wg-portal/wg-portal-*`
 - Link the system service file to the correct folder:
   - `sudo ln -s /opt/wg-portal/wg-portal.service /etc/systemd/system/wg-portal.service`
 - Reload the systemctl daemon:
   - `sudo systemctl daemon-reload`
    
### Manage the service
Once the service has been setup, you can simply manage the service using `systemctl`:
 - Enable on startup: `systemctl enable wg-portal.service`
 - Start: `systemctl start wg-portal.service`
 - Stop: `systemctl stop wg-portal.service`
 - Status: `systemctl status wg-portal.service`
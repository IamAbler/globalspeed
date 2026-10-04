# Desktop build assets

`appicon.png` is the desktop application icon. Wails converts it to the Windows executable icon and the macOS application bundle icon during `wails build`. Linux embeds the PNG for its window icon.

For a Linux application launcher, install the built `globalspeed-desktop` binary on your PATH, copy `linux/globalspeed.desktop` to `~/.local/share/applications/`, and copy `appicon.png` to `~/.local/share/icons/hicolor/512x512/apps/globalspeed.png`.

wingo-contrib is a small and simple program to install, upgrade and search 
scripts in the [contrib](https://github.com/wingowm/contrib) repository.
Scripts are installed in `$XDG_CONFIG_HOME/wingo/scripts`, which is
usually at `~/.config/wingo/scripts`.

## Installation

If you have Go installed, `go get` will work:

    go get github.com/wingowm/wingo-contrib

It's also in the
[Archlinux User Repository](https://aur.archlinux.org/packages/wingo-contrib-git/).


## Usage

    wingo-contrib command [arguments]
    
    The commands are:
    
        info       show information about script
        install    add scripts from wingo-contrib
        list       lists all installed scripts from wingo-contrib
        search     find scripts by searching descriptions
        upgrade    update a script


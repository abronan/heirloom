heirloom
========

Simple command line tool for the Pomodoro® technique

Great example for an exercise with Go using goroutines.

Based on the `say` command. Only tested on OSX. Be sure to check Enhanced Dictation for offline usage and download the voice *Samantha*.

###Installation

Be sure to have a proper Go installation. Then:

`go get github.com/abronan/heirloom`

go into the `hrl` folder and type : `go install`

### Basic usage

#####Add a task

`hrl add "Read the Raft algorithm research paper"`

#####Start a session

`hrl start` *using default timer values*

`hrl start --timer 25 --short 7 --long 30 --rounds 4` *overrides default values*
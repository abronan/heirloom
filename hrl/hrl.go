package main

import (
	"fmt"
	"strconv"
	"time"

	"os"
	"os/exec"

	"github.com/codegangsta/cli"
)

var (
	round       time.Duration = time.Second * 5
	shortBreak  time.Duration = time.Second * 2
	longBreak   time.Duration = time.Second * 4
	totalRounds int           = 5
	current     int           = 1
	task        *string       // TODO put into tiedot embedded database
)

func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %s", err.Error())
		os.Exit(1)
	}
}

func main() {
	app := cli.NewApp()
	app.Name = "hlr"
	app.Usage = "Simple command line tool for the Pomodoro® technique"
	app.Version = "0.0.1"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "timer, t",
			Value: "25",
			Usage: "time for a single pomodoro round",
		},
		cli.StringFlag{
			Name:  "short, s",
			Value: "5",
			Usage: "time for a short break",
		},
		cli.StringFlag{
			Name:  "long, l",
			Value: "20",
			Usage: "time for a long break",
		},
		cli.StringFlag{
			Name:  "rounds, r",
			Value: "1",
			Usage: "number of pomodoro rounds",
		},
	}
	app.Commands = []cli.Command{
		{
			Name:      "start",
			ShortName: "s",
			Usage:     "Start a pomodoro timer",
			Action: func(c *cli.Context) {
				// TODO handle cli flags

				pomodoro := make(chan bool)
				smallBreak := make(chan bool)
				longBreak := make(chan bool)
				done := make(chan bool)

				go pomodoroRound(pomodoro)
				go pomodoroService(pomodoro, smallBreak, longBreak, done)

				<-done
			},
		},
	}
	app.Run(os.Args)
}

func pomodoroRound(chanPomodoro chan bool) {
	tellBeginRound()
	time.Sleep(round)
	tellEndRound()
	chanPomodoro <- true
}

func pomodoroBreak(chanBreak chan bool) {
	tellBeginSmallBreak()
	time.Sleep(shortBreak)
	chanBreak <- true
}

func pomodoroLongBreak(chanLongBreak chan bool) {
	tellBeginLongBreak()
	time.Sleep(longBreak)
	chanLongBreak <- true
}

func pomodoroService(chanPomodoro, chanBreak, chanLongBreak, chanDone chan bool) {
	fmt.Printf("Pomodoro service started")
	for {
		select {

		case round := <-chanPomodoro:
			_ = round
			if current >= totalRounds {
				go pomodoroLongBreak(chanLongBreak)
				current = 1
				// TODO Ask if continue?
				chanDone <- true
			} else {
				current += 1
				go pomodoroBreak(chanBreak)
			}

		case smallBreak := <-chanBreak:
			_ = smallBreak
			tellEndSmallBreak()
			go pomodoroRound(chanPomodoro)

		case longBreak := <-chanLongBreak:
			_ = longBreak
			tellEndLongBreak()
			go pomodoroRound(chanPomodoro)

		}
	}
}

func tellBeginRound() {
	toSay := "Pomodoro round" + strconv.Itoa(current) + "begins"
	exec.Command("say", toSay).Output()
}

func tellEndRound() {
	exec.Command("say", "Round ended").Output()
	fmt.Printf("Have you finished the current task? (Y/N)")
	var input string
	fmt.Scanln(&input)
	// TODO Analyze input and handle task into tiedot
}

func tellBeginSmallBreak() {
	exec.Command("say", "Have a small break!").Output()
}

func tellEndSmallBreak() {
	exec.Command("say", "This is the end of the small break. Let's get back to work!").Output()
}

func tellBeginLongBreak() {
	exec.Command("say", "Have a long break! You deserved it!").Output()
}

func tellEndLongBreak() {
	exec.Command("say", "This is the end of the long break. Let's get back to work!").Output()
}

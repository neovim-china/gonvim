package editor

import (
	"sort"
	"sync"

	"github.com/therecipe/qt/widgets"
)

// Locpopup is the location popup
type Locpopup struct {
	ws           *Workspace
	mutex        sync.Mutex
	widget       *widgets.QWidget
	typeLabel    *widgets.QLabel
	typeText     string
	contentLabel *widgets.QLabel
	contentText  string
	shown        bool
	updates      chan []interface{}
}

func initLocpopup() *Locpopup {
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(8, 8, 8, 8)
	layout := widgets.NewQHBoxLayout()
	layout.SetContentsMargins(0, 0, 0, 0)
	layout.SetSpacing(4)
	widget.SetLayout(layout)
	widget.SetStyleSheet(".QWidget { border: 1px solid #000; } * {color: rgba(205, 211, 222, 1); background-color: rgba(24, 29, 34, 1);}")
	typeLabel := widgets.NewQLabel(nil, 0)
	typeLabel.SetContentsMargins(4, 1, 4, 1)

	contentLabel := widgets.NewQLabel(nil, 0)
	contentLabel.SetContentsMargins(0, 0, 0, 0)

	layout.AddWidget(typeLabel, 0, 0)
	layout.AddWidget(contentLabel, 0, 0)

	loc := &Locpopup{
		widget:       widget,
		typeLabel:    typeLabel,
		contentLabel: contentLabel,
		updates:      make(chan []interface{}, 1000),
	}
	return loc
}

func (l *Locpopup) subscribe() {
	if !l.ws.drawLint {
		return
	}
	l.ws.signal.ConnectLocpopupSignal(func() {
		l.updateLocpopup()
	})
	l.ws.nvim.RegisterHandler("LocPopup", func(args ...interface{}) {
		l.handle(args)
	})
	l.ws.nvim.Subscribe("LocPopup")
	l.ws.nvim.Command(`autocmd CursorMoved,CursorHold,InsertEnter,InsertLeave * call rpcnotify(0, "LocPopup", "update")`)
}

func (l *Locpopup) updateLocpopup() {
	if !l.shown {
		l.widget.Hide()
		return
	}
	l.contentLabel.SetText(l.contentText)
	if l.typeText == "E" {
		l.typeLabel.SetText("Error")
		l.typeLabel.SetStyleSheet("background-color: rgba(204, 62, 68, 1);")
	} else if l.typeText == "W" {
		l.typeLabel.SetText("Warning")
		l.typeLabel.SetStyleSheet("background-color: rgba(203, 203, 65, 1);")
	}
	l.widget.Hide()
	l.widget.Show()
}

func (l *Locpopup) handle(args []interface{}) {
	if len(args) < 1 {
		return
	}
	event, ok := args[0].(string)
	if !ok {
		return
	}
	switch event {
	case "update":
		l.update(args[1:])
	}
}

func (l *Locpopup) update(args []interface{}) {
	l.mutex.Lock()
	shown := false
	defer func() {
		if !shown {
			l.shown = false
			l.ws.signal.LocpopupSignal()
		}
		l.mutex.Unlock()
	}()
	buf, err := l.ws.nvim.CurrentBuffer()
	if err != nil {
		return
	}
	buftype := new(string)
	err = l.ws.nvim.BufferOption(buf, "buftype", buftype)
	if err != nil {
		return
	}
	if *buftype == "terminal" {
		return
	}

	mode := new(string)
	err = l.ws.nvim.Call("mode", mode, "")
	if err != nil {
		return
	}
	if *mode != "n" {
		return
	}

	curWin, err := l.ws.nvim.CurrentWindow()
	if err != nil {
		return
	}
	pos, err := l.ws.nvim.WindowCursor(curWin)
	if err != nil {
		return
	}
	result := new([]map[string]interface{})
	err = l.ws.nvim.Call("getloclist", result, "winnr(\"$\")")
	if err != nil {
		return
	}

	errors := 0
	warnings := 0
	locs := []map[string]interface{}{}
	for _, loc := range *result {
		lnumInterface := loc["lnum"]
		if lnumInterface == nil {
			continue
		}
		lnum := reflectToInt(lnumInterface)
		if lnum == pos[0] {
			locs = append(locs, loc)
		}
		locType := loc["type"].(string)
		switch locType {
		case "E":
			errors++
		case "W":
			warnings++
		}
	}
	l.ws.statusline.lint.redraw(errors, warnings)
	if len(locs) == 0 {
		return
	}
	if len(locs) > 1 {
		sort.Sort(ByCol(locs))
	}
	var loc map[string]interface{}
	for _, loc = range locs {
		if pos[1] >= reflectToInt(loc["col"])-1 {
			break
		}
	}

	locType := loc["type"].(string)
	text := loc["text"].(string)
	shown = true
	if locType != l.typeText || text != l.contentText || shown != l.shown {
		l.typeText = locType
		l.contentText = text
		l.shown = shown
		l.ws.signal.LocpopupSignal()
	}
}

// ByCol sorts locations by column
type ByCol []map[string]interface{}

// Len of locations
func (s ByCol) Len() int {
	return len(s)
}

// Swap locations
func (s ByCol) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less than
func (s ByCol) Less(i, j int) bool {
	return reflectToInt(s[i]["col"]) > reflectToInt(s[j]["col"])
}

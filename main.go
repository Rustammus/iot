package main

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

var (
	fanStatus          = false      // Состояние вентилятора (вкл/выкл)
	manualMode         = false      // Режим работы вентилятора (авто/ручной)
	manualModePPM      = false      // Режим работы датчика (авто/ручной)
	enableMQTT         = false      // Режим работы mqtt
	currentAirQA       atomic.Int64 // CO2 ppm
	qualityLabel       *gtk.Label
	fanStatusLabel     *gtk.Label
	qualitySpinButton  *gtk.SpinButton
	numSpin            *gtk.SpinButton
	gistSpin           *gtk.SpinButton
	intervalSpin       *gtk.SpinButton
	manualModeCheck    *gtk.CheckButton
	manualModePPMCheck *gtk.CheckButton

	sendTicker *time.Ticker
	mqc        *MQClient
)

// Симулирует измерение качества воздуха
func simulateAirQuality() {
	if manualModePPM {
		return
	}
	// Генерация случайного уровня CO2
	currentAirQ := int(currentAirQA.Load())
	if fanStatus {
		add := int(math.Abs(float64(currentAirQ-400)) / 20)
		currentAirQ -= rand.Intn(add)
	} else {
		add := int(math.Abs(float64(2000-currentAirQ)) / 40)
		currentAirQ += rand.Intn(add)
	}
	currentAirQA.Store(int64(currentAirQ))
}

// Обновляет состояние вентилятора
func updateFanStatus(airQuality int) {
	if manualMode {
		return // В ручном режиме вентилятор управляется вручную
	}
	num := int(numSpin.GetValue())
	gist := int(gistSpin.GetValue())

	if airQuality > num+gist {
		fanStatus = true
	} else if airQuality < num-gist {
		fanStatus = false
	}
}

// Обновляет интерфейс
func updateUI() {
	currentAirQ := int(currentAirQA.Load())
	qualityLabel.SetText(fmt.Sprintf("Текущий уровень CO2: %d ppm", currentAirQ))
	if fanStatus {
		fanStatusLabel.SetText("Вентилятор: Включен")
	} else {
		fanStatusLabel.SetText("Вентилятор: Выключен")
	}

}

// Основной цикл обновления данных
func startSimulation() {
	go func() {
		for {
			currentAirQ := int(currentAirQA.Load())
			simulateAirQuality()
			glib.IdleAdd(func() bool {
				updateFanStatus(currentAirQ)
				updateUI()
				return false // Возвращаем false, чтобы действие не повторялось
			})
			time.Sleep(1 * time.Second)
		}
	}()
}

// поток отправляющий сообщения MQTT
func startSending() func() {
	sendTicker = time.NewTicker(10 * time.Second)
	d := make(chan bool)

	go func() {
		for {
			select {
			case <-d:
				return
			case <-sendTicker.C:
				currentAirQ := int(currentAirQA.Load())
				mqc.SendAirQuality(currentAirQ)
			}
		}
	}()
	return func() {
		sendTicker.Stop()
		d <- true
	}
}

func receiveUpdates(ch <-chan string) {
	go func() {
		for {
			select {
			case v, ok := <-ch:
				if !ok {
					return
				}
				switch v {
				case "mode_auto":
					manualModeCheck.SetActive(false)
				case "mode_manual":
					manualModeCheck.SetActive(true)
				case "fan_on":
					if manualMode {
						fanStatus = true
					}
				case "fan_off":
					if manualMode {
						fanStatus = false
					}
				}
			}
		}
	}()
}

// Основная функция
func main() {
	currentAirQA.Store(1050)

	// MQTT
	mqc = NewMQ(false)
	defer mqc.c.Disconnect(1000)

	// Инициализация GTK
	gtk.Init(nil)

	// Создание окна
	win, _ := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	win.SetTitle("Умная вентиляция")
	win.SetDefaultSize(400, 200)
	win.Connect("destroy", func() {
		gtk.MainQuit()
	})

	// Главный контейнер
	box, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	win.Add(box)

	// Метка качества воздуха
	qualityLabel, _ = gtk.LabelNew("Текущий уровень CO2: 0 ppm")
	box.PackStart(qualityLabel, true, true, 5)

	qualitySpinButton, _ = gtk.SpinButtonNewWithRange(0, 5000, 1)
	qualitySpinButton.Connect("value_changed", func() {
		log.Println("qualitySpinButton value_changed")
		currentAirQA.Store(int64(qualitySpinButton.GetValueAsInt()))
	})
	box.PackStart(qualitySpinButton, false, false, 5)

	// Метка состояния вентилятора
	fanStatusLabel, _ = gtk.LabelNew("Вентилятор: Выключен")
	box.PackStart(fanStatusLabel, true, true, 5)

	// Переключатель ручного режима и mqtt
	checkBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	{
		manualModeCheck, _ = gtk.CheckButtonNewWithLabel("Ручной режим вентилятора")
		manualModeCheck.Connect("toggled", func() {
			manualMode = manualModeCheck.GetActive()
		})

		manualModePPMCheck, _ = gtk.CheckButtonNewWithLabel("Ручной режим датчика")
		manualModePPMCheck.Connect("toggled", func() {
			manualModePPM = manualModePPMCheck.GetActive()
			log.Print("qualitySpinButton visibility ", manualModePPM)
			qualitySpinButton.SetVisible(manualModePPM)
		})

		mqttModeCheck, _ := gtk.CheckButtonNewWithLabel("включить MQTT")
		mqttModeCheck.Connect("toggled", func() {
			enableMQTT = mqttModeCheck.GetActive()
			if enableMQTT {
				mqc.Connect()
			} else {
				mqc.Disconnect()
			}
		})

		checkBox.PackStart(manualModeCheck, true, false, 0)
		checkBox.PackStart(manualModePPMCheck, true, false, 0)
		checkBox.PackStart(mqttModeCheck, true, false, 0)
	}

	// SpinButtonsBox
	sBoxH, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 15)
	{
		// PPM box
		numSpinBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
		{
			numSpinL, _ := gtk.LabelNew("Целевой PPM")
			numSpin, _ = gtk.SpinButtonNewWithRange(400, 5000, 1)
			numSpin.SetValue(1100)
			numSpinBox.PackStart(numSpinL, true, false, 0)
			numSpinBox.PackStart(numSpin, true, false, 0)
		}

		// Gist box
		gistSpinBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
		{
			gistSpinL, _ := gtk.LabelNew("Гистерезис")
			gistSpin, _ = gtk.SpinButtonNewWithRange(0, 400, 1)
			gistSpin.SetValue(100)
			gistSpinBox.PackStart(gistSpinL, true, false, 0)
			gistSpinBox.PackStart(gistSpin, true, false, 0)
		}

		// Gist box
		intervalSpinBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
		{
			intervalSpinL, _ := gtk.LabelNew("Интервал передачи, с")
			intervalSpin, _ = gtk.SpinButtonNewWithRange(1, 3600, 1)
			intervalSpin.SetValue(10)
			intervalSpin.Connect("value_changed", func() {
				log.Println("intervalSpin value_changed")
				v := intervalSpin.GetValueAsInt()
				sendTicker.Reset(time.Duration(v) * time.Second)
			})
			intervalSpinBox.PackStart(intervalSpinL, true, false, 0)
			intervalSpinBox.PackStart(intervalSpin, true, false, 0)
		}
		sBoxH.PackStart(numSpinBox, true, false, 0)
		sBoxH.PackStart(gistSpinBox, true, false, 0)
		sBoxH.PackStart(intervalSpinBox, true, false, 0)
	}

	box.PackStart(sBoxH, true, false, 10)
	box.PackStart(checkBox, true, false, 10)

	// Кнопка для ручного включения/выключения вентилятора
	toggleFanButton, _ := gtk.ButtonNewWithLabel("Переключить вентилятор")
	toggleFanButton.Connect("clicked", func() {
		if manualMode {
			fanStatus = !fanStatus
			updateUI() // Обновление интерфейса
		}
	})
	box.PackStart(toggleFanButton, false, false, 5)

	// Запуск симуляции
	startSimulation()
	stop := startSending()
	defer stop()

	// Прием сообщений
	ch := make(chan string, 10)
	mqc.updateChan = ch
	receiveUpdates(ch)

	// Отображение окна
	win.ShowAll()
	qualitySpinButton.SetVisible(false)

	gtk.Main()
}

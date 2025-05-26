package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Stats struct {
	Total          int
	Success        int
	Failed         int
	OriginalSize   int64
	CompressedSize int64
	Errors         []string
	mu             sync.Mutex
}

func promptDirectory(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(prompt + ": ")
		path, _ := reader.ReadString('\n')
		path = strings.TrimSpace(path)
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			return path
		}
		fmt.Println("❌ Папка не найдена:", path, "\nПопробуйте снова.\n")
	}
}

func promptQuality() string {
	fmt.Println("Выберите уровень качества сжатия:")
	fmt.Println("1. screen  (низкое качество, высокая компрессия)")
	fmt.Println("2. ebook   (среднее качество, компромисс)")
	fmt.Println("3. printer (высокое качество, умеренная компрессия)")
	fmt.Println("4. prepress (максимальное качество, минимальная компрессия)")
	fmt.Println("5. default  (стандартные настройки)")
	fmt.Print("Введите номер (1-5, по умолчанию 2): ")
	reader := bufio.NewReader(os.Stdin)
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)
	switch choice {
	case "1":
		return "/screen"
	case "2", "":
		return "/ebook"
	case "3":
		return "/printer"
	case "4":
		return "/prepress"
	case "5":
		return "/default"
	default:
		return "/ebook"
	}
}

func findAllPDFs(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".pdf") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func compressPDF(input, output, quality string) (bool, int64, int64, string) {
	os.MkdirAll(filepath.Dir(output), os.ModePerm)
	gsCmd := []string{
		"-sDEVICE=pdfwrite",
		"-dCompatibilityLevel=1.4",
		fmt.Sprintf("-dPDFSETTINGS=%s", quality),
		"-dNOPAUSE",
		"-dQUIET",
		"-dBATCH",
		fmt.Sprintf("-sOutputFile=%s", output),
		input,
	}
	origInfo, err := os.Stat(input)
	if err != nil {
		return false, 0, 0, input
	}
	cmd := exec.Command("gswin64c", gsCmd...)
	err = cmd.Run()
	if err != nil {
		return false, 0, 0, input
	}
	compInfo, err := os.Stat(output)
	if err != nil {
		return false, 0, 0, input
	}
	return true, origInfo.Size(), compInfo.Size(), input
}

func checkGhostscript() {
	cmd := exec.Command("gswin64c", "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("❌ Ghostscript не найден. Убедитесь, что он установлен и добавлен в PATH.")
		os.Exit(1)
	}
	fmt.Printf("✔ Ghostscript найден. Версия: %s\n\n", strings.TrimSpace(string(out)))
}

func main() {
	fmt.Println("📁 Утилита пакетного сжатия PDF через Ghostscript (Go)\n")
	checkGhostscript()
	inputRoot := promptDirectory("Введите путь к ВХОДНОЙ директории")
	outputRoot := promptDirectory("Введите путь к ВЫХОДНОЙ директории")
	quality := promptQuality()
	start := time.Now()

	pdfFiles, err := findAllPDFs(inputRoot)
	if err != nil {
		fmt.Println("Ошибка поиска PDF:", err)
		return
	}
	stats := &Stats{Total: len(pdfFiles)}
	fmt.Printf("\n🔍 Найдено PDF-файлов: %d\n\n", stats.Total)

	var wg sync.WaitGroup
	for _, input := range pdfFiles {
		wg.Add(1)
		go func(input string) {
			defer wg.Done()
			rel, _ := filepath.Rel(inputRoot, input)
			output := filepath.Join(outputRoot, rel)
			ok, orig, comp, path := compressPDF(input, output, quality)
			stats.mu.Lock()
			defer stats.mu.Unlock()
			if ok {
				stats.Success++
				stats.OriginalSize += orig
				stats.CompressedSize += comp
			} else {
				stats.Failed++
				stats.Errors = append(stats.Errors, path)
			}
		}(input)
	}
	wg.Wait()

	fmt.Println("\n========================================")
	fmt.Println("📊 Сводка:")
	fmt.Printf("Всего обработано: %d\n", stats.Total)
	fmt.Printf("Успешно:          %d\n", stats.Success)
	fmt.Printf("С ошибками:       %d\n", stats.Failed)
	if stats.Total > 0 {
		saved := stats.OriginalSize - stats.CompressedSize
		ratio := float64(saved) / float64(stats.OriginalSize) * 100
		fmt.Printf("\n📉 Общий размер до:   %.2f MB\n", float64(stats.OriginalSize)/(1024*1024))
		fmt.Printf("📦 Общий размер после: %.2f MB\n", float64(stats.CompressedSize)/(1024*1024))
		fmt.Printf("💾 Экономия: %.2f MB (%.1f%%)\n", float64(saved)/(1024*1024), ratio)
	}
	if stats.Failed > 0 {
		fmt.Println("\n🚫 Ошибки в файлах:")
		for _, err := range stats.Errors {
			fmt.Println("  -", err)
		}
	}
	fmt.Println("========================================\n")
	fmt.Printf("⏱ Время выполнения: %s\n", time.Since(start))
	fmt.Println("👋 Спасибо за использование утилиты!")
	fmt.Println("\nНажмите Enter для выхода...")
	bufio.NewReader(os.Stdin).ReadBytes('\n')
}

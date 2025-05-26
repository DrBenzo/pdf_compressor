import os
import subprocess
from pathlib import Path
from typing import List, Tuple
from datetime import datetime
from concurrent.futures import ProcessPoolExecutor, as_completed
from functools import partial
from tqdm import tqdm

# ANSI-цвета для красивого вывода
class Color:
    GREEN = '\033[92m'
    RED = '\033[91m'
    CYAN = '\033[96m'
    YELLOW = '\033[93m'
    RESET = '\033[0m'

# Статистика выполнения
class Stats:
    total = 0
    success = 0
    failed = 0
    errors: List[str] = []
    original_size = 0
    compressed_size = 0

def prompt_directory(prompt_text: str) -> Path:
    while True:
        path = input(f"{prompt_text}: ").strip('" ').strip()
        if os.path.isdir(path):
            return Path(path)
        else:
            print(f"{Color.RED}❌ Папка не найдена: {path}{Color.RESET}\nПопробуйте снова.\n")

def prompt_quality() -> str:
    print("Выберите уровень качества сжатия:")
    print("1. screen  (низкое качество, высокая компрессия)")
    print("2. ebook   (среднее качество, компромисс)")
    print("3. printer (высокое качество, умеренная компрессия)")
    print("4. prepress (максимальное качество, минимальная компрессия)")
    print("5. default  (стандартные настройки)")
    choice = input("Введите номер (1-5, по умолчанию 2): ").strip()
    return {
        '1': '/screen',
        '2': '/ebook',
        '3': '/printer',
        '4': '/prepress',
        '5': '/default'
    }.get(choice, '/ebook')

def compress_pdf(input_path, output_path, quality) -> Tuple[bool, int, int, str]:
    output_path.parent.mkdir(parents=True, exist_ok=True)

    gs_command = [
        "gswin64c",
        "-sDEVICE=pdfwrite",
        "-dCompatibilityLevel=1.4",
        f"-dPDFSETTINGS={quality}",
        "-dNOPAUSE",
        "-dQUIET",
        "-dBATCH",
        f"-sOutputFile={str(output_path)}",
        str(input_path)
    ]

    try:
        original_size = os.path.getsize(input_path)
        subprocess.run(gs_command, check=True)
        compressed_size = os.path.getsize(output_path)
        return (True, original_size, compressed_size, str(input_path))
    except Exception:
        return (False, 0, 0, str(input_path))

def find_all_pdfs(root_dir: Path) -> List[Path]:
    return [Path(os.path.join(root, file))
            for root, _, files in os.walk(root_dir)
            for file in files if file.lower().endswith(".pdf")]

def process_all(input_dir, output_dir, quality):
    pdf_files = find_all_pdfs(input_dir)
    Stats.total = len(pdf_files)
    print(f"\n🔍 Найдено PDF-файлов: {Stats.total}\n")

    tasks = []
    for input_file in pdf_files:
        relative_path = input_file.relative_to(input_dir)
        output_file = output_dir / relative_path
        tasks.append((input_file, output_file))

    with ProcessPoolExecutor() as executor:
        futures = {
            executor.submit(compress_pdf, inp, out, quality): (inp, out)
            for inp, out in tasks
        }
        for future in tqdm(as_completed(futures), total=len(futures), desc="📦 Сжатие PDF", ncols=80):
            success, orig, comp, path = future.result()
            if success:
                Stats.success += 1
                Stats.original_size += orig
                Stats.compressed_size += comp
            else:
                Stats.failed += 1
                Stats.errors.append(path)

    print("\n" + "="*40)
    print(f"{Color.YELLOW}📊 Сводка:{Color.RESET}")
    print(f"Всего обработано: {Stats.total}")
    print(f"{Color.GREEN}Успешно:         {Stats.success}{Color.RESET}")
    print(f"{Color.RED}С ошибками:      {Stats.failed}{Color.RESET}")
    if Stats.total > 0:
        saved = Stats.original_size - Stats.compressed_size
        ratio = (saved / Stats.original_size) * 100 if Stats.original_size else 0
        print(f"\n📉 Общий размер до:   {Stats.original_size / (1024*1024):.2f} MB")
        print(f"📦 Общий размер после: {Stats.compressed_size / (1024*1024):.2f} MB")
        print(f"💾 Экономия: {saved / (1024*1024):.2f} MB ({ratio:.1f}%)")
    if Stats.failed > 0:
        print("\n🚫 Ошибки в файлах:")
        for err in Stats.errors:
            print(f"  - {err}")
    print("="*40 + "\n")

def check_ghostscript():
    try:
        result = subprocess.run([
            "gswin64c", "--version"],
            check=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True
        )
        print(f"{Color.GREEN}✔ Ghostscript найден. Версия: {result.stdout.strip()}{Color.RESET}\n")
    except FileNotFoundError:
        print(f"{Color.RED}❌ Ghostscript не найден. Убедитесь, что он установлен и добавлен в PATH.{Color.RESET}")
        exit(1)
    except subprocess.CalledProcessError as e:
        print(f"{Color.RED}❌ Ошибка при вызове Ghostscript:{Color.RESET} {e.stderr.strip()}")
        exit(1)

if __name__ == "__main__":
    print("📁 Утилита пакетного сжатия PDF через Ghostscript\n")
    check_ghostscript()
    input_root = prompt_directory("Введите путь к ВХОДНОЙ директории")
    output_root = prompt_directory("Введите путь к ВЫХОДНОЙ директории")
    quality = prompt_quality()
    start = datetime.now()
    process_all(input_root, output_root, quality)
    duration = datetime.now() - start
    print(f"⏱ Время выполнения: {duration}")
    print(f"👋 Спасибо за использование утилиты!")

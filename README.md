# Bulker - Parallel Processing Tool

Tool để chạy command line tools đa luồng thông qua tmux detach với khả năng chia file input và xử lý interrupt.

## Tính năng

- Chia file input thành chunks để xử lý parallel
- Chạy commands thông qua tmux detach (background)
- Hỗ trợ đa luồng với giới hạn số workers
- Interrupt handling để thu thập kết quả partial
- Merge kết quả từ các chunks
- Cleanup mode để thu thập kết quả từ run bị gián đoạn

## Cài đặt

```bash
go mod tidy
go build -o bulker
```

## Sử dụng

### Cú pháp cơ bản

```bash
./bulker run [command] --input [input_file] [options]
```

### Tham số

- `--input, -i`: File input (required)
- `--output, -o`: Thư mục output (default: "output")
- `--workers, -w`: Số workers parallel (default: 4)
- `--chunk-size, -c`: Kích thước chunk (số lines) (default: 1000)
- `--session, -s`: Tên tmux session (default: "bulker")
- `--cleanup`: Chế độ cleanup - thu thập kết quả từ run bị gián đoạn

### Placeholders trong command

- `{input}`: Được thay thế bằng đường dẫn chunk file
- `{output}`: Được thay thế bằng đường dẫn result file

### Ví dụ

#### Chạy grep parallel

```bash
./bulker run grep --input data.txt --workers 8 --chunk-size 500 -- -i "pattern" {input} > {output}
```

#### Chạy custom processing script

```bash
./bulker run python --input big_data.txt --workers 4 -- process.py {input} {output}
```

#### Cleanup sau khi bị interrupt

```bash
./bulker run --cleanup --output output_dir
```

## Workflow

1. Tool chia file input thành chunks
2. Tạo tmux session mới
3. Chạy command trên từng chunk trong tmux windows riêng biệt
4. Monitor progress và báo cáo status
5. Khi hoàn thành, merge tất cả result files
6. Nếu bị interrupt (Ctrl+C), cleanup và merge kết quả partial

## Cấu trúc output

```
output/
├── chunk_0000.txt    # Input chunks
├── chunk_0001.txt
├── result_0000.txt   # Result files
├── result_0001.txt
└── merged_result.txt # Final merged result
```

## Quản lý tmux

Tool tạo một tmux session với tên được chỉ định (default: "bulker") và tạo windows riêng cho mỗi worker. Bạn có thể:

```bash
# Xem sessions
tmux list-sessions

# Attach vào session để monitor
tmux attach-session -t bulker

# Xem các windows
tmux list-windows -t bulker
```

## Xử lý lỗi

- Nếu tmux không có sẵn, tool sẽ báo lỗi
- Nếu file input không tồn tại, tool sẽ báo lỗi
- Nếu command thất bại trên một chunk, tool sẽ báo warning và tiếp tục
- Khi nhận interrupt signal, tool sẽ cleanup và merge kết quả đã có 
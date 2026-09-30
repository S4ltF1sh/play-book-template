# Chào mừng đến với Playbook Template

Đây là **chương demo** — nó tồn tại để bạn thấy mọi loại nội dung của template hoạt động ra sao, và để agent sinh nội dung có mẫu đối chiếu. Khi playbook thật của bạn sẵn sàng, chỉ cần thay thư mục `content/` và build lại.

## Một playbook gồm những gì?

| Loại section | Cách mở khóa bài tiếp theo |
|--------------|---------------------------|
| **Đọc** (như trang này) | bấm "Đánh dấu đã học" |
| **Quiz xen kẽ** | trả lời đúng *tất cả* câu hỏi (được làm lại) |
| **Bài tập** | chạy code đạt đủ tiêu chí chấm tự động |

Cuối mỗi chương luôn có **Tóm tắt & Thuật ngữ** và **Trắc nghiệm** tổng kết.

## Playground bên phải

- Mỗi bài tập có 1 hoặc 2 pane code, tùy đề bài. Hai pane chạy **độc lập** — đủ để mô phỏng client/server.
- Code chạy **thật trên máy bạn** qua pty: gõ input ở ô dưới cùng, truyền tham số ở ô `args:`.
- Tab **Scratch** luôn có sẵn để thử nhanh bất kỳ đoạn code nào.
- Toolchain là dữ liệu: `content/toolchains.json` mô tả cách mỗi pane build và chạy. Template có sẵn preset (`c`, `cpp`, `python`, `node`, `kotlin`, `java`, `rust`, `rust-cargo`) và playbook có thể tự định nghĩa thêm (make, Gradle, Python venv, …) — mỗi pane khai báo toolchain riêng trong `exercises.json`.

> Thử ngay: sang phần quiz rồi bài tập ở mục kế tiếp — bạn sẽ đi qua đủ ba cơ chế mở khóa.

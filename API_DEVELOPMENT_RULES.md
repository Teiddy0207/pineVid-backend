# 📜 QUY TẮC BẮT BUỘC KHI VIẾT BẤT KỲ API NÀO (API_DEVELOPMENT_RULES.md)

**Dự án:** PipeVid Microservices  
**Áp dụng cho:** Tất cả Developer khi phát triển API trên Go Backend (`backend/`)  
**Trạng thái:** BẮT BUỘC TUÂN THỦ 100% (MANDATORY)  

---

## 🛑 6 QUY TẮC VÀNG BẮT BUỘC (THE 6 GOLDEN RULES)

```text
[HTTP Client / Frontend]
          │ (Request DTO JSON)
          ▼
┌─────────────────────────────────────────────────────────────┐
│ 1. CONTROLLER (REST API Handler)                             │
│    - Parse Request DTO                                      │
│    - Không chứa Business Logic, Không đụng Entity/Mapper    │
│    - Gọi: resDTO, err := usecase.DoSomething(ctx, reqDTO)   │
│    - Trả: ctx.JSON(resDTO)                                  │
└──────────────────────────────┬──────────────────────────────┘
                               │ (Request DTO)
                               ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. USECASE (Business Logic Container)                       │
│    - Gọi Mapper: entity := mapper.ToEntity(reqDTO)          │
│    - Gọi Repo: repo.Store(ctx, entity)                      │
│    - Gọi Repo: repo.GetByID(ctx, id) -> entity              │
│    - Gọi Mapper: resDTO := mapper.ToResponse(entity)        │
│    - Trả: resDTO về cho Controller                          │
└──────────────────────────────┬──────────────────────────────┘
                               │ (Entity)
                               ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. REPOSITORY (PostgreSQL / Squirrel SQL)                   │
│    - Chỉ thao tác với Entity và Database SQL                │
└─────────────────────────────────────────────────────────────┘
```

---

### 1. QUY TẮC 1: CONTROLLER SIÊU MỎNG (THIN CONTROLLER)
- Controller chỉ đóng vai trò Transport Layer tiếp nhận HTTP.
- **NGHIÊM CẤM:** Không viết Business Logic, không viết câu lệnh SQL, KHÔNG ĐƯỢC chứa code mapping biến thể (`a.X = b.X`), và KHÔNG ĐƯỢC gọi package `mapper` hay chạm vào `entity`.
- Controller nhận **Request DTO** từ HTTP Client, truyền sang Usecase, nhận lại **Response DTO** từ Usecase và render JSON.

### 2. QUY TẮC 2: USECASE LÀM CHỦ PHÂN LUỒNG VÀ GỌI MAPPER
- Usecase là nơi **duy nhất** đứng ra điều phối và gọi `mapper`.
- Khi ghi dữ liệu: Usecase nhận Request DTO từ Controller -> Gọi `mapper.ToEntity(reqDTO)` -> Truyền `entity` sang Repo.
- Khi đọc dữ liệu: Usecase nhận `entity` từ Repo -> Gọi `mapper.ToResponse(entity)` -> Trả Response DTO về cho Controller.

### 3. QUY TẮC 3: TÁCH BIỆT DTO VÀ ENTITY (ZERO RAW ENTITY EXPOSURE)
- **TUYỆT ĐỐI KHÔNG** trả về raw `entity` CSDL ra ngoài API cho Frontend.
- Mọi API đọc/ghi đều phải có Struct **Request DTO** (đặt trong `internal/controller/restapi/v1/request/`) và Struct **Response DTO** (đặt trong `internal/controller/restapi/v1/response/`).

### 4. QUY TẮC 4: QUẢN LÝ MAPPING TRONG FOLDER MAPPER ĐỘC LẬP
- Toàn bộ hàm chuyển đổi dữ liệu bắt buộc phải nằm trong package `internal/mapper/` (ví dụ: `mapper/video.go`, `mapper/user.go`).
- Quy chuẩn tên hàm mapping:
  - `To<Model>Entity(req DTO) -> entity.<Model>`
  - `To<Model>Response(entity) -> response.<Model>Response`
  - `To<Model>PageResponse(entities, total, page, limit) -> response.PageResponse[...]`

### 5. QUY TẮC 5: CHUẨN HÓA PHÂN TRANG (PAGINATION STANDARD)
- Mọi API danh sách bắt buộc nhận 2 Query Params: `page` (mặc định = 1) và `limit` (mặc định = 10, tối đa = 50).
- Usecase nhận `page`, `limit` -> tự tính `offset = (page - 1) * limit`.
- Định dạng JSON trả về cho API phân trang **BẮT BUỘC** dùng struct `response.PageResponse[T]`:
  ```json
  {
    "success": true,
    "data": [ ... ],
    "pagination": {
      "total_items": 142,
      "total_pages": 15,
      "current_page": 1,
      "limit": 10
    }
  }
  ```

### 6. QUY TẮC 6: CHUẨN HÓA MÃ LỖI (ERROR RESPONSE STANDARD)
- Sử dụng đúng HTTP Status Code (`200 OK`, `201 Created`, `400 Bad Request`, `401 Unauthorized`, `403 Forbidden`, `404 Not Found`, `409 Conflict`, `500 Internal Error`).
- Mọi trường hợp lỗi đều trả về định dạng JSON thống nhất từ `errorResponse(ctx, code, message)`:
  ```json
  {
    "success": false,
    "code": 400,
    "error": "Mô tả chi tiết nguyên nhân lỗi"
  }
  ```

---

## 📋 CHECKLIST RÀ SOÁT KHI REVIEW CODE MỘT API MỚI

Trước khi Tạo Pull Request / Commit bất kỳ API mới nào, hãy tự kiểm tra 5 câu hỏi sau:

- [ ] 1. Controller của bạn có gọi package `mapper` hoặc dùng `entity` không? *(Nếu CÓ -> SAI QUY TẮC 1, hãy chuyển việc gọi Mapper vào Usecase)*.
- [ ] 2. API có trả về raw `entity` CSDL trực tiếp ra ngoài không? *(Nếu CÓ -> SAI QUY TẮC 3, hãy tạo Response DTO)*.
- [ ] 3. Hàm mapping có bị viết lộn xộn trong file Controller/Usecase không? *(Nếu CÓ -> SAI QUY TẮC 4, hãy di chuyển vào `internal/mapper/`)*.
- [ ] 4. API danh sách đã có `page`, `limit` và cấu trúc `pagination` chưa? *(Nếu CHƯA -> SAI QUY TẮC 5)*.
- [ ] 5. Mọi trường hợp lỗi đã được xử lý bằng HTTP Status Code và `errorResponse` chuẩn chưa? *(Nếu CHƯA -> SAI QUY TẮC 6)*.

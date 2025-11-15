# 🎫 PUSTIK Helpdesk Ticketing System  
A real-time ticketing web application built for managing electronic device complaints at PUSTIK (Pusat Teknologi Informasi).  
This system includes:

✔ User page for submitting complaints  
✔ Admin dashboard for monitoring & managing tickets in real-time  
✔ Full CRUD operations  
✔ WebSocket live updates (admin sees changes instantly)  
✔ Backend powered by Go  
✔ MySQL database (Laragon or standalone MySQL)  
✔ Frontend using HTML + CSS + JavaScript  

---
## Creator
Isaac Yeremia / 223400016
Kevin Handoyo / 223400006


## 📁 Project Structure


---

# 🚀 Features

### 🧑‍💻 User
- Submit complaint (name, phone, room, description, status, priority)
- Automatic ticket creation

### 👨‍🏫 Admin
- View all tickets in real-time
- Edit tickets via popup form
- Delete tickets
- Auto-update dashboard via WebSocket connection
- No page refresh required

### ⚙ Backend (Go)
- REST API for tickets (`GET`, `POST`, `PUT`, `DELETE`)
- WebSocket server for admin panel (`/ws/admin`)
- MySQL database integration
- Clean and modular code

---

# 🛠 Requirements

Before running this project, make sure you have:

- **Go 1.19+**
- **MySQL / MariaDB** (Laragon recommended on Windows)
- **Git**
- **Browser (Chrome/Edge/Firefox)**

---

# 📥 Database Setup (Laragon / MySQL)

1. Start Laragon → **Start All**
2. Open phpMyAdmin → `http://localhost/phpmyadmin`
3. Create database:


CREATE DATABASE ticketing_db;

4. Import SQL file:
- Go to **Import**
- Select: `db/ticketing_db.sql`
- Click **Go**

Database now ready.

---

# 🏃 Running the Backend (Go)

1. Navigate to backend folder:

```bash
cd backend

go mod tidy

go run main.go -dsn "root:@tcp(127.0.0.1:3306)/ticketing_db?parseTime=true" -static ../static -addr ":8080"

go run main.go -dsn "root:YOURPASSWORD@tcp(127.0.0.1:3306)/ticketing_db?parseTime=true" -static ../static -addr ":8080"

go run main.go -dsn "root:@tcp(127.0.0.1:3306)/ticketing_db?parseTime=true" -static ../static -addr ":8081"


Accessing the Web App
User Page (Submit Complaint)
http://localhost:8080/index.html

Admin Dashboard (Real-time View)
http://localhost:8080/admin.html

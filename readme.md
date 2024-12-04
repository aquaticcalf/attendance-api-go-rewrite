## attendance api go rewrite

### overview
this api provides endpoints for managing student attendance data. it allows fetching individual attendance, comparing attendance across multiple students, and calculating attendance percentage after skipping hours.

### base url
```
http://localhost:5000
```

---

## endpoints

### 1. **get attendance**
retrieve attendance data for a single student.

**endpoint:**  
```
get /attendance
```

**query parameters:**  

| parameter   | type   | required | description                     |
|-------------|--------|----------|---------------------------------|
| `student_id`| string | yes      | the unique id of the student.  |
| `password`  | string | yes      | the student's password.        |

**response:**  
- **success (200):** returns the attendance data as json.
- **error (400):** `{"error": "missing student_id or password"}`
- **error (500):** `{"error": "failed to fetch attendance data"}`

**example request:**  
```
get /attendance?student_id=12345&password=abcde
```

---

### 2. **compare attendance**
compare attendance data across multiple students.

**endpoint:**  
```
post /compare
```

**request body:**  
a json array of student credentials.  
```json
[
  {"student_id": "12345", "password": "abcde"},
  {"student_id": "67890", "password": "xyz"}
]
```

**response:**  
- **success (200):**  
  ```json
  {
    "students": [
      {
        "student_id": "12345",
        "total_attended": 45,
        "total_held": 50,
        "total_percentage": "90%",
        "hours_status": 5
      }
    ],
    "subject_points_summary": {
      "mathematics": {
        "max_percentage": 95,
        "top_students": ["12345"]
      }
    }
  }
  ```
- **error (400):** `{"error": "invalid input. expecting a list of student credentials."}`

---

### 3. **calculate attendance after skip**
calculate a student's new attendance percentage after skipping hours.

**endpoint:**  
```
get /skip
```

**query parameters:**  
| parameter   | type    | required | description                        |
|-------------|---------|----------|------------------------------------|
| `student_id`| string  | yes      | the unique id of the student.     |
| `password`  | string  | yes      | the student's password.           |
| `hours`     | integer | yes      | number of hours the student plans to skip. |

**response:**  
- **success (200):**  
  if skipping is safe:
  ```json
  {
    "original_attendance_percentage": "90%",
    "new_attendance_percentage": 87.5,
    "status": "safe to skip",
    "hours_can_skip_after": 3
  }
  ```
  if skipping requires attending more:
  ```json
  {
    "original_attendance_percentage": "90%",
    "new_attendance_percentage": 74,
    "status": "needs to attend more",
    "additional_hours_needed_after": 2
  }
  ```
- **error (400):** `{"error": "missing student_id, password, or hours"}`
- **error (500):** `{"error": "failed to fetch attendance data"}`

**example request:**  
```
get /skip?student_id=12345&password=abcde&hours=2
```

---

## error codes

| http code | message                                   | description                          |
|-----------|-------------------------------------------|--------------------------------------|
| 400       | `{"error": "missing ... or invalid input"}`| indicates missing or incorrect parameters. |
| 500       | `{"error": "failed to ... data"}`         | internal server or data processing errors.|

---

## notes
- the api communicates in json format for both requests and responses.
- ensure secure handling of sensitive student credentials.
- this server does not implement user authentication; it assumes valid credentials are provided.

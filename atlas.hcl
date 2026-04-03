env "local" {
  src = "file://internal/adapter/postgres/migrations" # Migration source directory
  url = "postgres://platform:secret@localhost:5432/remnacore?sslmode=disable" # Target database

  migration {
    dir = "file://internal/adapter/postgres/migrations" # Directory to store migration files
  }
}

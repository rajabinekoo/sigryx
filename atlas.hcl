env "local" {
  src = "ent://internal/ent/schema?dialect=postgres"
  dev = "docker://postgres/16/test"
  url = getenv("POSTGRES_DSN")
  schemas = ["vault"]
  migration {
    dir = "file://migrations"
  }
  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}

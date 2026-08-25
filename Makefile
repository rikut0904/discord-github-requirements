DATABASE_URL ?= postgresql://bot:bot_password@localhost:5432/github_bot?sslmode=disable

migrate:
	psql "$(DATABASE_URL)" -f migrations/001_create_user_settings.sql
	psql "$(DATABASE_URL)" -f migrations/002_create_user_notification_channels.sql
	psql "$(DATABASE_URL)" -f migrations/003_add_dependabot_settings.sql
#!/bin/bash
set -eu

TIMEZONE=Europe/Vienna
USERNAME=greenlight
PASSWORD=pa55word
DB_NAME=greenlight
DB_USER=greenlight
DB_PASSWORD=pa55word

# Reference-only provisioning script for a fresh Ubuntu/Debian server, following
# "Let's Go Further" chapter 21 (Deployment and Hosting). Not executed as part
# of this build — run manually on a real production host if/when you deploy.

echo "$USERNAME:$PASSWORD" | chpasswd
timedatectl set-timezone "$TIMEZONE"

apt update
apt --yes upgrade
apt --yes install postgresql caddy

useradd --create-home --shell "/bin/bash" --groups sudo "$USERNAME"

su - postgres -c "psql -c \"CREATE DATABASE $DB_NAME\""
su - postgres -c "psql -d $DB_NAME -c \"CREATE EXTENSION IF NOT EXISTS citext\""
su - postgres -c "psql -d $DB_NAME -c \"CREATE ROLE $DB_USER WITH LOGIN PASSWORD '$DB_PASSWORD'\""

echo "GREENLIGHT_DB_DSN='postgres://$DB_USER:$DB_PASSWORD@localhost/$DB_NAME'" >> /etc/environment

ufw allow 22
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable

echo "Setup complete. Reboot to apply changes."

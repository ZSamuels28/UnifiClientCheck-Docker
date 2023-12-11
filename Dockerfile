# Use an official PHP runtime as a parent image
FROM php:latest

# Install system dependencies
RUN apt-get update && apt-get install -y \
    git \
    unzip \
    libzip-dev \
    && rm -rf /var/lib/apt/lists/*

# Install PHP extensions
RUN docker-php-ext-install pdo pdo_mysql zip

# Set the working directory in the container
WORKDIR /usr/src/myapp

# Install Composer globally
RUN curl -sS https://getcomposer.org/installer | php -- --install-dir=/usr/bin --filename=composer

# Copy the current directory contents into the container at /usr/src/myapp
COPY . /usr/src/myapp

# Install project dependencies with Composer
RUN composer install --no-scripts

# Run the script when the container launches
CMD [ "php", "/usr/src/myapp/src/UniFiClientAlerts.php" ]
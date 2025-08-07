# Multi-stage build for Java Spring Boot User Management Service
FROM maven:3.8-openjdk-17-slim AS builder

# Set working directory
WORKDIR /app

# Copy pom.xml first for better caching
COPY pom.xml .

# Download dependencies
RUN mvn dependency:go-offline -B

# Copy source code
COPY src ./src

# Build the application
RUN mvn clean package -DskipTests

# Production stage
FROM eclipse-temurin:17-jre-alpine AS production

# Create app directory and user
WORKDIR /app
RUN addgroup -g 1001 -S spring && \
    adduser -S -u 1001 -G spring spring

# Create logs directory
RUN mkdir -p logs && chown -R spring:spring logs

# Copy the built jar from builder stage
COPY --from=builder /app/target/*.jar app.jar

# Change ownership to spring user
RUN chown -R spring:spring /app

# Switch to non-root user
USER spring

# Expose port
EXPOSE 8081

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=40s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8081/actuator/health || exit 1

# Start the application
ENTRYPOINT ["java", "-jar", "app.jar"]

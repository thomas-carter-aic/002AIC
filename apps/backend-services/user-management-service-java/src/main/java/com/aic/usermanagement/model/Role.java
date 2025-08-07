package com.aic.usermanagement.model;

public enum Role {
    ADMIN("Administrator with full system access"),
    USER("Regular user with basic access"),
    DEVELOPER("Developer with development tools access"),
    DATA_SCIENTIST("Data scientist with analytics access"),
    MODEL_MANAGER("AI/ML model management access"),
    VIEWER("Read-only access to resources");

    private final String description;

    Role(String description) {
        this.description = description;
    }

    public String getDescription() {
        return description;
    }

    public String getAuthority() {
        return "ROLE_" + this.name();
    }
}

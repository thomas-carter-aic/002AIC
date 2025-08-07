package com.aic.usermanagement.model;

import com.fasterxml.jackson.annotation.JsonIgnore;
import lombok.Data;
import lombok.EqualsAndHashCode;
import org.springframework.data.annotation.CreatedDate;
import org.springframework.data.annotation.LastModifiedDate;
import org.springframework.data.jpa.domain.support.AuditingEntityListener;

import jakarta.persistence.*;
import jakarta.validation.constraints.Email;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;
import java.time.LocalDateTime;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;

@Entity
@Table(name = "users", indexes = {
    @Index(name = "idx_user_email", columnList = "email"),
    @Index(name = "idx_user_username", columnList = "username"),
    @Index(name = "idx_user_keycloak_id", columnList = "keycloak_id")
})
@EntityListeners(AuditingEntityListener.class)
@Data
@EqualsAndHashCode(callSuper = false)
public class User {
    
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;
    
    @Column(name = "keycloak_id", unique = true, nullable = false)
    @NotBlank(message = "Keycloak ID is required")
    private String keycloakId;
    
    @Column(name = "username", unique = true, nullable = false)
    @NotBlank(message = "Username is required")
    @Size(min = 3, max = 50, message = "Username must be between 3 and 50 characters")
    private String username;
    
    @Column(name = "email", unique = true, nullable = false)
    @NotBlank(message = "Email is required")
    @Email(message = "Email should be valid")
    private String email;
    
    @Column(name = "first_name")
    @Size(max = 50, message = "First name must not exceed 50 characters")
    private String firstName;
    
    @Column(name = "last_name")
    @Size(max = 50, message = "Last name must not exceed 50 characters")
    private String lastName;
    
    @Column(name = "display_name")
    @Size(max = 100, message = "Display name must not exceed 100 characters")
    private String displayName;
    
    @Column(name = "avatar_url")
    private String avatarUrl;
    
    @Column(name = "phone_number")
    private String phoneNumber;
    
    @Column(name = "company")
    private String company;
    
    @Column(name = "job_title")
    private String jobTitle;
    
    @Column(name = "department")
    private String department;
    
    @Column(name = "location")
    private String location;
    
    @Column(name = "timezone")
    private String timezone;
    
    @Column(name = "language", length = 10)
    private String language = "en";
    
    @Column(name = "is_active")
    private Boolean isActive = true;
    
    @Column(name = "is_verified")
    private Boolean isVerified = false;
    
    @Column(name = "email_verified")
    private Boolean emailVerified = false;
    
    @Column(name = "phone_verified")
    private Boolean phoneVerified = false;
    
    @Column(name = "two_factor_enabled")
    private Boolean twoFactorEnabled = false;
    
    @Column(name = "last_login")
    private LocalDateTime lastLogin;
    
    @Column(name = "login_count")
    private Long loginCount = 0L;
    
    @Column(name = "failed_login_attempts")
    private Integer failedLoginAttempts = 0;
    
    @Column(name = "account_locked_until")
    private LocalDateTime accountLockedUntil;
    
    @Column(name = "password_changed_at")
    private LocalDateTime passwordChangedAt;
    
    @Column(name = "terms_accepted_at")
    private LocalDateTime termsAcceptedAt;
    
    @Column(name = "privacy_policy_accepted_at")
    private LocalDateTime privacyPolicyAcceptedAt;
    
    @ElementCollection
    @CollectionTable(name = "user_roles", joinColumns = @JoinColumn(name = "user_id"))
    @Column(name = "role")
    private Set<String> roles = new HashSet<>();
    
    @ElementCollection
    @CollectionTable(name = "user_permissions", joinColumns = @JoinColumn(name = "user_id"))
    @Column(name = "permission")
    private Set<String> permissions = new HashSet<>();
    
    @ElementCollection
    @CollectionTable(name = "user_tags", joinColumns = @JoinColumn(name = "user_id"))
    @Column(name = "tag")
    private Set<String> tags = new HashSet<>();
    
    @ElementCollection
    @CollectionTable(name = "user_metadata", joinColumns = @JoinColumn(name = "user_id"))
    @MapKeyColumn(name = "metadata_key")
    @Column(name = "metadata_value")
    private Map<String, String> metadata;
    
    @OneToMany(mappedBy = "user", cascade = CascadeType.ALL, fetch = FetchType.LAZY)
    @JsonIgnore
    private Set<UserSession> sessions = new HashSet<>();
    
    @OneToMany(mappedBy = "user", cascade = CascadeType.ALL, fetch = FetchType.LAZY)
    @JsonIgnore
    private Set<UserPreference> preferences = new HashSet<>();
    
    @OneToMany(mappedBy = "user", cascade = CascadeType.ALL, fetch = FetchType.LAZY)
    @JsonIgnore
    private Set<UserActivity> activities = new HashSet<>();
    
    @CreatedDate
    @Column(name = "created_at", nullable = false, updatable = false)
    private LocalDateTime createdAt;
    
    @LastModifiedDate
    @Column(name = "updated_at")
    private LocalDateTime updatedAt;
    
    @Column(name = "created_by")
    private String createdBy;
    
    @Column(name = "updated_by")
    private String updatedBy;
    
    // Computed fields
    @Transient
    public String getFullName() {
        if (firstName != null && lastName != null) {
            return firstName + " " + lastName;
        } else if (firstName != null) {
            return firstName;
        } else if (lastName != null) {
            return lastName;
        }
        return username;
    }
    
    @Transient
    public Boolean isAccountLocked() {
        return accountLockedUntil != null && accountLockedUntil.isAfter(LocalDateTime.now());
    }
    
    @Transient
    public Boolean hasRole(String role) {
        return roles != null && roles.contains(role);
    }
    
    @Transient
    public Boolean hasPermission(String permission) {
        return permissions != null && permissions.contains(permission);
    }
    
    @Transient
    public Boolean hasAnyRole(Set<String> rolesToCheck) {
        if (roles == null || rolesToCheck == null) {
            return false;
        }
        return roles.stream().anyMatch(rolesToCheck::contains);
    }
    
    @PrePersist
    protected void onCreate() {
        if (displayName == null) {
            displayName = getFullName();
        }
    }
    
    @PreUpdate
    protected void onUpdate() {
        if (displayName == null) {
            displayName = getFullName();
        }
    }
}

package com.aic.marketplace.model;

import com.fasterxml.jackson.annotation.JsonIgnore;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.ToString;
import org.hibernate.annotations.CreationTimestamp;
import org.hibernate.annotations.UpdateTimestamp;

import javax.persistence.*;
import javax.validation.constraints.*;
import java.math.BigDecimal;
import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;

@Entity
@Table(name = "plugins")
@Data
@EqualsAndHashCode(exclude = {"versions", "installations", "reviews"})
@ToString(exclude = {"versions", "installations", "reviews"})
public class Plugin {

    @Id
    @Column(name = "id", length = 36)
    private String id;

    @NotBlank
    @Size(max = 255)
    @Column(name = "name", nullable = false)
    private String name;

    @NotBlank
    @Size(max = 100)
    @Column(name = "slug", nullable = false, unique = true)
    private String slug;

    @NotBlank
    @Size(max = 1000)
    @Column(name = "description", nullable = false)
    private String description;

    @Column(name = "long_description", columnDefinition = "TEXT")
    private String longDescription;

    @NotBlank
    @Size(max = 100)
    @Column(name = "category", nullable = false)
    private String category;

    @ElementCollection
    @CollectionTable(name = "plugin_tags", joinColumns = @JoinColumn(name = "plugin_id"))
    @Column(name = "tag")
    private List<String> tags = new ArrayList<>();

    @NotBlank
    @Size(max = 255)
    @Column(name = "developer_id", nullable = false)
    private String developerId;

    @NotBlank
    @Size(max = 255)
    @Column(name = "developer_name", nullable = false)
    private String developerName;

    @Email
    @Size(max = 255)
    @Column(name = "developer_email")
    private String developerEmail;

    @Size(max = 500)
    @Column(name = "website_url")
    private String websiteUrl;

    @Size(max = 500)
    @Column(name = "repository_url")
    private String repositoryUrl;

    @Size(max = 500)
    @Column(name = "documentation_url")
    private String documentationUrl;

    @Size(max = 500)
    @Column(name = "support_url")
    private String supportUrl;

    @Size(max = 500)
    @Column(name = "icon_url")
    private String iconUrl;

    @ElementCollection
    @CollectionTable(name = "plugin_screenshots", joinColumns = @JoinColumn(name = "plugin_id"))
    @Column(name = "screenshot_url")
    private List<String> screenshots = new ArrayList<>();

    @Column(name = "price", precision = 10, scale = 2)
    private BigDecimal price = BigDecimal.ZERO;

    @Size(max = 3)
    @Column(name = "currency", length = 3)
    private String currency = "USD";

    @Size(max = 50)
    @Column(name = "pricing_model")
    @Enumerated(EnumType.STRING)
    private PricingModel pricingModel = PricingModel.FREE;

    @Size(max = 50)
    @Column(name = "status")
    @Enumerated(EnumType.STRING)
    private PluginStatus status = PluginStatus.DRAFT;

    @Column(name = "is_featured")
    private Boolean isFeatured = false;

    @Column(name = "is_verified")
    private Boolean isVerified = false;

    @Column(name = "download_count")
    private Long downloadCount = 0L;

    @Column(name = "install_count")
    private Long installCount = 0L;

    @Column(name = "rating", precision = 3, scale = 2)
    private BigDecimal rating = BigDecimal.ZERO;

    @Column(name = "review_count")
    private Integer reviewCount = 0;

    @Size(max = 50)
    @Column(name = "latest_version")
    private String latestVersion;

    @Column(name = "min_platform_version")
    private String minPlatformVersion;

    @Column(name = "max_platform_version")
    private String maxPlatformVersion;

    @ElementCollection
    @CollectionTable(name = "plugin_permissions", joinColumns = @JoinColumn(name = "plugin_id"))
    @Column(name = "permission")
    private List<String> permissions = new ArrayList<>();

    @ElementCollection
    @MapKeyColumn(name = "key")
    @Column(name = "value")
    @CollectionTable(name = "plugin_metadata", joinColumns = @JoinColumn(name = "plugin_id"))
    private Map<String, String> metadata;

    @Column(name = "revenue_share_percent", precision = 5, scale = 2)
    private BigDecimal revenueSharePercent = new BigDecimal("30.00");

    @Column(name = "published_at")
    private LocalDateTime publishedAt;

    @CreationTimestamp
    @Column(name = "created_at", nullable = false, updatable = false)
    private LocalDateTime createdAt;

    @UpdateTimestamp
    @Column(name = "updated_at", nullable = false)
    private LocalDateTime updatedAt;

    @OneToMany(mappedBy = "plugin", cascade = CascadeType.ALL, fetch = FetchType.LAZY)
    @JsonIgnore
    private List<PluginVersion> versions = new ArrayList<>();

    @OneToMany(mappedBy = "plugin", cascade = CascadeType.ALL, fetch = FetchType.LAZY)
    @JsonIgnore
    private List<PluginInstallation> installations = new ArrayList<>();

    @OneToMany(mappedBy = "plugin", cascade = CascadeType.ALL, fetch = FetchType.LAZY)
    @JsonIgnore
    private List<PluginReview> reviews = new ArrayList<>();

    public enum PricingModel {
        FREE, ONE_TIME, SUBSCRIPTION, FREEMIUM, PAY_PER_USE
    }

    public enum PluginStatus {
        DRAFT, PENDING_REVIEW, APPROVED, PUBLISHED, REJECTED, SUSPENDED, ARCHIVED
    }

    // Helper methods
    public boolean isFree() {
        return pricingModel == PricingModel.FREE || 
               (price != null && price.compareTo(BigDecimal.ZERO) == 0);
    }

    public boolean isPublished() {
        return status == PluginStatus.PUBLISHED;
    }

    public void incrementDownloadCount() {
        this.downloadCount = (this.downloadCount == null ? 0L : this.downloadCount) + 1;
    }

    public void incrementInstallCount() {
        this.installCount = (this.installCount == null ? 0L : this.installCount) + 1;
    }

    public void updateRating(BigDecimal newRating, int newReviewCount) {
        this.rating = newRating;
        this.reviewCount = newReviewCount;
    }
}

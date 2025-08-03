package com.aic.marketplace.controller;

import com.aic.marketplace.dto.*;
import com.aic.marketplace.model.Plugin;
import com.aic.marketplace.model.PluginInstallation;
import com.aic.marketplace.model.PluginReview;
import com.aic.marketplace.model.PluginVersion;
import com.aic.marketplace.service.MarketplaceService;
import com.aic.marketplace.service.PluginInstallationService;
import com.aic.marketplace.service.PluginReviewService;
import com.aic.marketplace.service.PluginVersionService;
import io.micrometer.core.annotation.Timed;
import io.swagger.v3.oas.annotations.Operation;
import io.swagger.v3.oas.annotations.Parameter;
import io.swagger.v3.oas.annotations.tags.Tag;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.validation.annotation.Validated;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.multipart.MultipartFile;

import javax.validation.Valid;
import javax.validation.constraints.NotBlank;
import java.util.List;
import java.util.Map;

@RestController
@RequestMapping("/v1")
@RequiredArgsConstructor
@Validated
@Slf4j
@Tag(name = "Marketplace", description = "Plugin marketplace and extension management API")
public class MarketplaceController {

    private final MarketplaceService marketplaceService;
    private final PluginVersionService pluginVersionService;
    private final PluginInstallationService installationService;
    private final PluginReviewService reviewService;

    // Health check
    @GetMapping("/health")
    @Operation(summary = "Health check", description = "Check service health status")
    public ResponseEntity<Map<String, Object>> healthCheck() {
        return ResponseEntity.ok(marketplaceService.getHealthStatus());
    }

    // Plugin Management
    @PostMapping("/plugins")
    @Operation(summary = "Create plugin", description = "Create a new plugin listing")
    @Timed(value = "marketplace.plugin.create", description = "Time taken to create plugin")
    public ResponseEntity<PluginResponse> createPlugin(
            @Valid @RequestBody CreatePluginRequest request) {
        log.info("Creating plugin: {}", request.getName());
        Plugin plugin = marketplaceService.createPlugin(request);
        return ResponseEntity.status(HttpStatus.CREATED)
                .body(PluginResponse.fromEntity(plugin));
    }

    @GetMapping("/plugins/{pluginId}")
    @Operation(summary = "Get plugin", description = "Retrieve plugin by ID")
    public ResponseEntity<PluginResponse> getPlugin(
            @Parameter(description = "Plugin ID") @PathVariable @NotBlank String pluginId) {
        Plugin plugin = marketplaceService.getPlugin(pluginId);
        return ResponseEntity.ok(PluginResponse.fromEntity(plugin));
    }

    @GetMapping("/plugins")
    @Operation(summary = "List plugins", description = "List plugins with filtering and pagination")
    public ResponseEntity<Page<PluginResponse>> listPlugins(
            @Parameter(description = "Category filter") @RequestParam(required = false) String category,
            @Parameter(description = "Tag filter") @RequestParam(required = false) String tag,
            @Parameter(description = "Search query") @RequestParam(required = false) String search,
            @Parameter(description = "Pricing model filter") @RequestParam(required = false) String pricingModel,
            @Parameter(description = "Status filter") @RequestParam(required = false) String status,
            @Parameter(description = "Featured only") @RequestParam(defaultValue = "false") boolean featured,
            @Parameter(description = "Verified only") @RequestParam(defaultValue = "false") boolean verified,
            @Parameter(description = "Sort by") @RequestParam(defaultValue = "created_at") String sortBy,
            @Parameter(description = "Sort direction") @RequestParam(defaultValue = "desc") String sortDir,
            Pageable pageable) {
        
        PluginSearchRequest searchRequest = PluginSearchRequest.builder()
                .category(category)
                .tag(tag)
                .search(search)
                .pricingModel(pricingModel)
                .status(status)
                .featured(featured)
                .verified(verified)
                .sortBy(sortBy)
                .sortDir(sortDir)
                .build();

        Page<Plugin> plugins = marketplaceService.searchPlugins(searchRequest, pageable);
        Page<PluginResponse> response = plugins.map(PluginResponse::fromEntity);
        return ResponseEntity.ok(response);
    }

    @PutMapping("/plugins/{pluginId}")
    @Operation(summary = "Update plugin", description = "Update plugin information")
    @Timed(value = "marketplace.plugin.update", description = "Time taken to update plugin")
    public ResponseEntity<PluginResponse> updatePlugin(
            @Parameter(description = "Plugin ID") @PathVariable @NotBlank String pluginId,
            @Valid @RequestBody UpdatePluginRequest request) {
        log.info("Updating plugin: {}", pluginId);
        Plugin plugin = marketplaceService.updatePlugin(pluginId, request);
        return ResponseEntity.ok(PluginResponse.fromEntity(plugin));
    }

    @DeleteMapping("/plugins/{pluginId}")
    @Operation(summary = "Delete plugin", description = "Delete a plugin")
    @Timed(value = "marketplace.plugin.delete", description = "Time taken to delete plugin")
    public ResponseEntity<Void> deletePlugin(
            @Parameter(description = "Plugin ID") @PathVariable @NotBlank String pluginId) {
        log.info("Deleting plugin: {}", pluginId);
        marketplaceService.deletePlugin(pluginId);
        return ResponseEntity.noContent().build();
    }

    @PostMapping("/plugins/{pluginId}/publish")
    @Operation(summary = "Publish plugin", description = "Publish a plugin to the marketplace")
    @Timed(value = "marketplace.plugin.publish", description = "Time taken to publish plugin")
    public ResponseEntity<PluginResponse> publishPlugin(
            @Parameter(description = "Plugin ID") @PathVariable @NotBlank String pluginId) {
        log.info("Publishing plugin: {}", pluginId);
        Plugin plugin = marketplaceService.publishPlugin(pluginId);
        return ResponseEntity.ok(PluginResponse.fromEntity(plugin));
    }

    @PostMapping("/plugins/{pluginId}/unpublish")
    @Operation(summary = "Unpublish plugin", description = "Unpublish a plugin from the marketplace")
    @Timed(value = "marketplace.plugin.unpublish", description = "Time taken to unpublish plugin")
    public ResponseEntity<PluginResponse> unpublishPlugin(
            @Parameter(description = "Plugin ID") @PathVariable @NotBlank String pluginId) {
        log.info("Unpublishing plugin: {}", pluginId);
        Plugin plugin = marketplaceService.unpublishPlugin(pluginId);
        return ResponseEntity.ok(PluginResponse.fromEntity(plugin));
    }

    // Plugin Version Management
    @PostMapping("/plugins/{pluginId}/versions")
    @Operation(summary = "Create plugin version", description = "Create a new version of a plugin")
    @Timed(value = "marketplace.version.create", description = "Time taken to create plugin version")
    public ResponseEntity<PluginVersionResponse> createPluginVersion(
            @Parameter(description = "Plugin ID") @PathVariable @NotBlank String pluginId,
            @Valid @RequestBody CreatePluginVersionRequest request) {
        log.info("Creating version {} for plugin: {}", request.getVersion(), pluginId);
        PluginVersion version = pluginVersionService.createVersion(pluginId, request);
        return ResponseEntity.status(HttpStatus.CREATED)
                .body(PluginVersionResponse.fromEntity(version));
    }

    @GetMapping("/plugins/{pluginId}/versions")
    @Operation(summary = "List plugin versions", description = "List all versions of a plugin")
    public ResponseEntity<List<PluginVersionResponse>> listPluginVersions(
            @Parameter(description = "Plugin ID") @PathVariable @NotBlank String pluginId) {
        List<PluginVersion> versions = pluginVersionService.listVersions(pluginId);
        List<PluginVersionResponse> response = versions.stream()
                .map(PluginVersionResponse::fromEntity)
                .toList();
        return ResponseEntity.ok(response);
    }

    @GetMapping("/plugins/{pluginId}/versions/{version}")
    @Operation(summary = "Get plugin version", description = "Get specific version of a plugin")
    public ResponseEntity<PluginVersionResponse> getPluginVersion(
            @Parameter(description = "Plugin ID") @PathVariable @NotBlank String pluginId,
            @Parameter(description = "Version") @PathVariable @NotBlank String version) {
        PluginVersion pluginVersion = pluginVersionService.getVersion(pluginId, version);
        return ResponseEntity.ok(PluginVersionResponse.fromEntity(pluginVersion));
    }

    @PostMapping("/plugins/{pluginId}/versions/{version}/upload")
    @Operation(summary = "Upload plugin package", description = "Upload plugin package file")
    @Timed(value = "marketplace.version.upload", description = "Time taken to upload plugin package")
    public ResponseEntity<PluginVersionResponse> uploadPluginPackage(
            @Parameter(description = "Plugin ID") @PathVariable @NotBlank String pluginId,
            @Parameter(description = "Version") @PathVariable @NotBlank String version,
            @RequestParam("file") MultipartFile file) {
        log.info("Uploading package for plugin {} version {}", pluginId, version);
        PluginVersion pluginVersion = pluginVersionService.uploadPackage(pluginId, version, file);
        return ResponseEntity.ok(PluginVersionResponse.fromEntity(pluginVersion));
    }

    @GetMapping("/plugins/{pluginId}/versions/{version}/download")
    @Operation(summary = "Download plugin package", description = "Download plugin package file")
    @Timed(value = "marketplace.version.download", description = "Time taken to download plugin package")
    public ResponseEntity<byte[]> downloadPluginPackage(
            @Parameter(description = "Plugin ID") @PathVariable @NotBlank String pluginId,
            @Parameter(description = "Version") @PathVariable @NotBlank String version) {
        log.info("Downloading package for plugin {} version {}", pluginId, version);
        byte[] packageData = pluginVersionService.downloadPackage(pluginId, version);
        
        return ResponseEntity.ok()
                .header("Content-Disposition", "attachment; filename=\"" + pluginId + "-" + version + ".zip\"")
                .header("Content-Type", "application/zip")
                .body(packageData);
    }

    // Plugin Installation Management
    @PostMapping("/plugins/{pluginId}/install")
    @Operation(summary = "Install plugin", description = "Install a plugin for a user")
    @Timed(value = "marketplace.plugin.install", description = "Time taken to install plugin")
    public ResponseEntity<PluginInstallationResponse> installPlugin(
            @Parameter(description = "Plugin ID") @PathVariable @NotBlank String pluginId,
            @Valid @RequestBody InstallPluginRequest request) {
        log.info("Installing plugin {} for user {}", pluginId, request.getUserId());
        PluginInstallation installation = installationService.installPlugin(pluginId, request);
        return ResponseEntity.status(HttpStatus.CREATED)
                .body(PluginInstallationResponse.fromEntity(installation));
    }

    @DeleteMapping("/plugins/{pluginId}/uninstall")
    @Operation(summary = "Uninstall plugin", description = "Uninstall a plugin for a user")
    @Timed(value = "marketplace.plugin.uninstall", description = "Time taken to uninstall plugin")
    public ResponseEntity<Void> uninstallPlugin(
            @Parameter(description = "Plugin ID") @PathVariable @NotBlank String pluginId,
            @Parameter(description = "User ID") @RequestParam @NotBlank String userId) {
        log.info("Uninstalling plugin {} for user {}", pluginId, userId);
        installationService.uninstallPlugin(pluginId, userId);
        return ResponseEntity.noContent().build();
    }

    @GetMapping("/users/{userId}/installations")
    @Operation(summary = "List user installations", description = "List all plugin installations for a user")
    public ResponseEntity<List<PluginInstallationResponse>> listUserInstallations(
            @Parameter(description = "User ID") @PathVariable @NotBlank String userId) {
        List<PluginInstallation> installations = installationService.listUserInstallations(userId);
        List<PluginInstallationResponse> response = installations.stream()
                .map(PluginInstallationResponse::fromEntity)
                .toList();
        return ResponseEntity.ok(response);
    }

    @PostMapping("/plugins/{pluginId}/installations/{installationId}/update")
    @Operation(summary = "Update plugin installation", description = "Update a plugin installation")
    @Timed(value = "marketplace.installation.update", description = "Time taken to update installation")
    public ResponseEntity<PluginInstallationResponse> updatePluginInstallation(
            @Parameter(description = "Plugin ID") @PathVariable @NotBlank String pluginId,
            @Parameter(description = "Installation ID") @PathVariable @NotBlank String installationId,
            @Valid @RequestBody UpdateInstallationRequest request) {
        log.info("Updating installation {} for plugin {}", installationId, pluginId);
        PluginInstallation installation = installationService.updateInstallation(installationId, request);
        return ResponseEntity.ok(PluginInstallationResponse.fromEntity(installation));
    }

    // Plugin Reviews
    @PostMapping("/plugins/{pluginId}/reviews")
    @Operation(summary = "Create review", description = "Create a review for a plugin")
    @Timed(value = "marketplace.review.create", description = "Time taken to create review")
    public ResponseEntity<PluginReviewResponse> createReview(
            @Parameter(description = "Plugin ID") @PathVariable @NotBlank String pluginId,
            @Valid @RequestBody CreateReviewRequest request) {
        log.info("Creating review for plugin {} by user {}", pluginId, request.getUserId());
        PluginReview review = reviewService.createReview(pluginId, request);
        return ResponseEntity.status(HttpStatus.CREATED)
                .body(PluginReviewResponse.fromEntity(review));
    }

    @GetMapping("/plugins/{pluginId}/reviews")
    @Operation(summary = "List plugin reviews", description = "List reviews for a plugin")
    public ResponseEntity<Page<PluginReviewResponse>> listPluginReviews(
            @Parameter(description = "Plugin ID") @PathVariable @NotBlank String pluginId,
            @Parameter(description = "Rating filter") @RequestParam(required = false) Integer rating,
            Pageable pageable) {
        Page<PluginReview> reviews = reviewService.listPluginReviews(pluginId, rating, pageable);
        Page<PluginReviewResponse> response = reviews.map(PluginReviewResponse::fromEntity);
        return ResponseEntity.ok(response);
    }

    @PutMapping("/reviews/{reviewId}")
    @Operation(summary = "Update review", description = "Update a plugin review")
    @Timed(value = "marketplace.review.update", description = "Time taken to update review")
    public ResponseEntity<PluginReviewResponse> updateReview(
            @Parameter(description = "Review ID") @PathVariable @NotBlank String reviewId,
            @Valid @RequestBody UpdateReviewRequest request) {
        log.info("Updating review: {}", reviewId);
        PluginReview review = reviewService.updateReview(reviewId, request);
        return ResponseEntity.ok(PluginReviewResponse.fromEntity(review));
    }

    @DeleteMapping("/reviews/{reviewId}")
    @Operation(summary = "Delete review", description = "Delete a plugin review")
    @Timed(value = "marketplace.review.delete", description = "Time taken to delete review")
    public ResponseEntity<Void> deleteReview(
            @Parameter(description = "Review ID") @PathVariable @NotBlank String reviewId) {
        log.info("Deleting review: {}", reviewId);
        reviewService.deleteReview(reviewId);
        return ResponseEntity.noContent().build();
    }

    // Categories and Tags
    @GetMapping("/categories")
    @Operation(summary = "List categories", description = "List all plugin categories")
    public ResponseEntity<List<CategoryResponse>> listCategories() {
        List<CategoryResponse> categories = marketplaceService.listCategories();
        return ResponseEntity.ok(categories);
    }

    @GetMapping("/tags")
    @Operation(summary = "List tags", description = "List popular plugin tags")
    public ResponseEntity<List<TagResponse>> listTags(
            @Parameter(description = "Limit") @RequestParam(defaultValue = "50") int limit) {
        List<TagResponse> tags = marketplaceService.listPopularTags(limit);
        return ResponseEntity.ok(tags);
    }

    // Analytics
    @GetMapping("/analytics/plugins/{pluginId}")
    @Operation(summary = "Plugin analytics", description = "Get analytics for a plugin")
    public ResponseEntity<PluginAnalyticsResponse> getPluginAnalytics(
            @Parameter(description = "Plugin ID") @PathVariable @NotBlank String pluginId,
            @Parameter(description = "Start date (YYYY-MM-DD)") @RequestParam(required = false) String startDate,
            @Parameter(description = "End date (YYYY-MM-DD)") @RequestParam(required = false) String endDate) {
        PluginAnalyticsResponse analytics = marketplaceService.getPluginAnalytics(pluginId, startDate, endDate);
        return ResponseEntity.ok(analytics);
    }

    @GetMapping("/analytics/marketplace")
    @Operation(summary = "Marketplace analytics", description = "Get overall marketplace analytics")
    public ResponseEntity<MarketplaceAnalyticsResponse> getMarketplaceAnalytics(
            @Parameter(description = "Start date (YYYY-MM-DD)") @RequestParam(required = false) String startDate,
            @Parameter(description = "End date (YYYY-MM-DD)") @RequestParam(required = false) String endDate) {
        MarketplaceAnalyticsResponse analytics = marketplaceService.getMarketplaceAnalytics(startDate, endDate);
        return ResponseEntity.ok(analytics);
    }

    @GetMapping("/analytics/developers/{developerId}")
    @Operation(summary = "Developer analytics", description = "Get analytics for a developer")
    public ResponseEntity<DeveloperAnalyticsResponse> getDeveloperAnalytics(
            @Parameter(description = "Developer ID") @PathVariable @NotBlank String developerId,
            @Parameter(description = "Start date (YYYY-MM-DD)") @RequestParam(required = false) String startDate,
            @Parameter(description = "End date (YYYY-MM-DD)") @RequestParam(required = false) String endDate) {
        DeveloperAnalyticsResponse analytics = marketplaceService.getDeveloperAnalytics(developerId, startDate, endDate);
        return ResponseEntity.ok(analytics);
    }

    // Revenue Management
    @GetMapping("/revenue/plugins/{pluginId}")
    @Operation(summary = "Plugin revenue", description = "Get revenue information for a plugin")
    public ResponseEntity<PluginRevenueResponse> getPluginRevenue(
            @Parameter(description = "Plugin ID") @PathVariable @NotBlank String pluginId,
            @Parameter(description = "Start date (YYYY-MM-DD)") @RequestParam(required = false) String startDate,
            @Parameter(description = "End date (YYYY-MM-DD)") @RequestParam(required = false) String endDate) {
        PluginRevenueResponse revenue = marketplaceService.getPluginRevenue(pluginId, startDate, endDate);
        return ResponseEntity.ok(revenue);
    }

    @GetMapping("/revenue/developers/{developerId}")
    @Operation(summary = "Developer revenue", description = "Get revenue information for a developer")
    public ResponseEntity<DeveloperRevenueResponse> getDeveloperRevenue(
            @Parameter(description = "Developer ID") @PathVariable @NotBlank String developerId,
            @Parameter(description = "Start date (YYYY-MM-DD)") @RequestParam(required = false) String startDate,
            @Parameter(description = "End date (YYYY-MM-DD)") @RequestParam(required = false) String endDate) {
        DeveloperRevenueResponse revenue = marketplaceService.getDeveloperRevenue(developerId, startDate, endDate);
        return ResponseEntity.ok(revenue);
    }

    // Featured and Trending
    @GetMapping("/featured")
    @Operation(summary = "Featured plugins", description = "Get featured plugins")
    public ResponseEntity<List<PluginResponse>> getFeaturedPlugins(
            @Parameter(description = "Limit") @RequestParam(defaultValue = "10") int limit) {
        List<Plugin> plugins = marketplaceService.getFeaturedPlugins(limit);
        List<PluginResponse> response = plugins.stream()
                .map(PluginResponse::fromEntity)
                .toList();
        return ResponseEntity.ok(response);
    }

    @GetMapping("/trending")
    @Operation(summary = "Trending plugins", description = "Get trending plugins")
    public ResponseEntity<List<PluginResponse>> getTrendingPlugins(
            @Parameter(description = "Limit") @RequestParam(defaultValue = "10") int limit,
            @Parameter(description = "Period (7d, 30d, 90d)") @RequestParam(defaultValue = "7d") String period) {
        List<Plugin> plugins = marketplaceService.getTrendingPlugins(limit, period);
        List<PluginResponse> response = plugins.stream()
                .map(PluginResponse::fromEntity)
                .toList();
        return ResponseEntity.ok(response);
    }

    @GetMapping("/recommendations/{userId}")
    @Operation(summary = "Plugin recommendations", description = "Get personalized plugin recommendations")
    public ResponseEntity<List<PluginResponse>> getRecommendations(
            @Parameter(description = "User ID") @PathVariable @NotBlank String userId,
            @Parameter(description = "Limit") @RequestParam(defaultValue = "10") int limit) {
        List<Plugin> plugins = marketplaceService.getRecommendations(userId, limit);
        List<PluginResponse> response = plugins.stream()
                .map(PluginResponse::fromEntity)
                .toList();
        return ResponseEntity.ok(response);
    }
}

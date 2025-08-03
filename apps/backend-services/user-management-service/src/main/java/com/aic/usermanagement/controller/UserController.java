package com.aic.usermanagement.controller;

import com.aic.usermanagement.dto.*;
import com.aic.usermanagement.model.User;
import com.aic.usermanagement.service.UserService;
import io.micrometer.core.annotation.Timed;
import io.swagger.v3.oas.annotations.Operation;
import io.swagger.v3.oas.annotations.Parameter;
import io.swagger.v3.oas.annotations.responses.ApiResponse;
import io.swagger.v3.oas.annotations.responses.ApiResponses;
import io.swagger.v3.oas.annotations.security.SecurityRequirement;
import io.swagger.v3.oas.annotations.tags.Tag;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.security.core.Authentication;
import org.springframework.validation.annotation.Validated;
import org.springframework.web.bind.annotation.*;

import javax.validation.Valid;
import javax.validation.constraints.NotBlank;
import java.util.List;
import java.util.Map;
import java.util.Set;

@RestController
@RequestMapping("/v1/users")
@RequiredArgsConstructor
@Slf4j
@Validated
@Tag(name = "User Management", description = "User management operations")
@SecurityRequirement(name = "bearerAuth")
public class UserController {

    private final UserService userService;

    @GetMapping("/health")
    @Operation(summary = "Health check", description = "Check service health")
    public ResponseEntity<Map<String, Object>> health() {
        return ResponseEntity.ok(Map.of(
            "status", "healthy",
            "service", "user-management-service",
            "timestamp", java.time.Instant.now().toString(),
            "version", "1.0.0"
        ));
    }

    @GetMapping
    @PreAuthorize("hasRole('ADMIN') or hasPermission('user', 'read')")
    @Operation(summary = "List users", description = "Get paginated list of users")
    @ApiResponses({
        @ApiResponse(responseCode = "200", description = "Users retrieved successfully"),
        @ApiResponse(responseCode = "403", description = "Access denied")
    })
    @Timed(value = "user.list", description = "Time taken to list users")
    public ResponseEntity<Page<UserResponseDto>> listUsers(
            @Parameter(description = "Search query") @RequestParam(required = false) String search,
            @Parameter(description = "Filter by role") @RequestParam(required = false) String role,
            @Parameter(description = "Filter by active status") @RequestParam(required = false) Boolean active,
            @Parameter(description = "Filter by verified status") @RequestParam(required = false) Boolean verified,
            Pageable pageable) {
        
        log.info("Listing users with search: {}, role: {}, active: {}, verified: {}", 
                search, role, active, verified);
        
        Page<UserResponseDto> users = userService.findUsers(search, role, active, verified, pageable);
        return ResponseEntity.ok(users);
    }

    @GetMapping("/{id}")
    @PreAuthorize("hasRole('ADMIN') or hasPermission('user', 'read') or #id == authentication.name")
    @Operation(summary = "Get user by ID", description = "Retrieve user details by ID")
    @ApiResponses({
        @ApiResponse(responseCode = "200", description = "User found"),
        @ApiResponse(responseCode = "404", description = "User not found"),
        @ApiResponse(responseCode = "403", description = "Access denied")
    })
    @Timed(value = "user.get", description = "Time taken to get user")
    public ResponseEntity<UserResponseDto> getUser(
            @Parameter(description = "User ID") @PathVariable Long id) {
        
        log.info("Getting user with ID: {}", id);
        UserResponseDto user = userService.findById(id);
        return ResponseEntity.ok(user);
    }

    @GetMapping("/keycloak/{keycloakId}")
    @PreAuthorize("hasRole('ADMIN') or hasPermission('user', 'read')")
    @Operation(summary = "Get user by Keycloak ID", description = "Retrieve user details by Keycloak ID")
    @Timed(value = "user.get.keycloak", description = "Time taken to get user by Keycloak ID")
    public ResponseEntity<UserResponseDto> getUserByKeycloakId(
            @Parameter(description = "Keycloak ID") @PathVariable String keycloakId) {
        
        log.info("Getting user with Keycloak ID: {}", keycloakId);
        UserResponseDto user = userService.findByKeycloakId(keycloakId);
        return ResponseEntity.ok(user);
    }

    @GetMapping("/me")
    @Operation(summary = "Get current user", description = "Get current authenticated user's profile")
    @Timed(value = "user.me", description = "Time taken to get current user")
    public ResponseEntity<UserResponseDto> getCurrentUser(Authentication authentication) {
        log.info("Getting current user profile for: {}", authentication.getName());
        UserResponseDto user = userService.findByKeycloakId(authentication.getName());
        return ResponseEntity.ok(user);
    }

    @PostMapping
    @PreAuthorize("hasRole('ADMIN') or hasPermission('user', 'create')")
    @Operation(summary = "Create user", description = "Create a new user")
    @ApiResponses({
        @ApiResponse(responseCode = "201", description = "User created successfully"),
        @ApiResponse(responseCode = "400", description = "Invalid input"),
        @ApiResponse(responseCode = "409", description = "User already exists")
    })
    @Timed(value = "user.create", description = "Time taken to create user")
    public ResponseEntity<UserResponseDto> createUser(
            @Valid @RequestBody CreateUserRequestDto request,
            Authentication authentication) {
        
        log.info("Creating user with username: {}", request.getUsername());
        UserResponseDto user = userService.createUser(request, authentication.getName());
        return ResponseEntity.status(HttpStatus.CREATED).body(user);
    }

    @PutMapping("/{id}")
    @PreAuthorize("hasRole('ADMIN') or hasPermission('user', 'update') or #id == authentication.name")
    @Operation(summary = "Update user", description = "Update user details")
    @ApiResponses({
        @ApiResponse(responseCode = "200", description = "User updated successfully"),
        @ApiResponse(responseCode = "404", description = "User not found"),
        @ApiResponse(responseCode = "400", description = "Invalid input")
    })
    @Timed(value = "user.update", description = "Time taken to update user")
    public ResponseEntity<UserResponseDto> updateUser(
            @Parameter(description = "User ID") @PathVariable Long id,
            @Valid @RequestBody UpdateUserRequestDto request,
            Authentication authentication) {
        
        log.info("Updating user with ID: {}", id);
        UserResponseDto user = userService.updateUser(id, request, authentication.getName());
        return ResponseEntity.ok(user);
    }

    @PutMapping("/me")
    @Operation(summary = "Update current user", description = "Update current user's profile")
    @Timed(value = "user.update.me", description = "Time taken to update current user")
    public ResponseEntity<UserResponseDto> updateCurrentUser(
            @Valid @RequestBody UpdateUserRequestDto request,
            Authentication authentication) {
        
        log.info("Updating current user profile for: {}", authentication.getName());
        User currentUser = userService.findUserByKeycloakId(authentication.getName());
        UserResponseDto user = userService.updateUser(currentUser.getId(), request, authentication.getName());
        return ResponseEntity.ok(user);
    }

    @DeleteMapping("/{id}")
    @PreAuthorize("hasRole('ADMIN') or hasPermission('user', 'delete')")
    @Operation(summary = "Delete user", description = "Delete user (soft delete)")
    @ApiResponses({
        @ApiResponse(responseCode = "204", description = "User deleted successfully"),
        @ApiResponse(responseCode = "404", description = "User not found")
    })
    @Timed(value = "user.delete", description = "Time taken to delete user")
    public ResponseEntity<Void> deleteUser(
            @Parameter(description = "User ID") @PathVariable Long id,
            Authentication authentication) {
        
        log.info("Deleting user with ID: {}", id);
        userService.deleteUser(id, authentication.getName());
        return ResponseEntity.noContent().build();
    }

    @PostMapping("/{id}/activate")
    @PreAuthorize("hasRole('ADMIN') or hasPermission('user', 'update')")
    @Operation(summary = "Activate user", description = "Activate a deactivated user")
    @Timed(value = "user.activate", description = "Time taken to activate user")
    public ResponseEntity<UserResponseDto> activateUser(
            @Parameter(description = "User ID") @PathVariable Long id,
            Authentication authentication) {
        
        log.info("Activating user with ID: {}", id);
        UserResponseDto user = userService.activateUser(id, authentication.getName());
        return ResponseEntity.ok(user);
    }

    @PostMapping("/{id}/deactivate")
    @PreAuthorize("hasRole('ADMIN') or hasPermission('user', 'update')")
    @Operation(summary = "Deactivate user", description = "Deactivate a user")
    @Timed(value = "user.deactivate", description = "Time taken to deactivate user")
    public ResponseEntity<UserResponseDto> deactivateUser(
            @Parameter(description = "User ID") @PathVariable Long id,
            Authentication authentication) {
        
        log.info("Deactivating user with ID: {}", id);
        UserResponseDto user = userService.deactivateUser(id, authentication.getName());
        return ResponseEntity.ok(user);
    }

    @PostMapping("/{id}/roles")
    @PreAuthorize("hasRole('ADMIN') or hasPermission('user', 'update')")
    @Operation(summary = "Assign roles", description = "Assign roles to user")
    @Timed(value = "user.assign.roles", description = "Time taken to assign roles")
    public ResponseEntity<UserResponseDto> assignRoles(
            @Parameter(description = "User ID") @PathVariable Long id,
            @Valid @RequestBody AssignRolesRequestDto request,
            Authentication authentication) {
        
        log.info("Assigning roles {} to user with ID: {}", request.getRoles(), id);
        UserResponseDto user = userService.assignRoles(id, request.getRoles(), authentication.getName());
        return ResponseEntity.ok(user);
    }

    @DeleteMapping("/{id}/roles")
    @PreAuthorize("hasRole('ADMIN') or hasPermission('user', 'update')")
    @Operation(summary = "Remove roles", description = "Remove roles from user")
    @Timed(value = "user.remove.roles", description = "Time taken to remove roles")
    public ResponseEntity<UserResponseDto> removeRoles(
            @Parameter(description = "User ID") @PathVariable Long id,
            @Valid @RequestBody RemoveRolesRequestDto request,
            Authentication authentication) {
        
        log.info("Removing roles {} from user with ID: {}", request.getRoles(), id);
        UserResponseDto user = userService.removeRoles(id, request.getRoles(), authentication.getName());
        return ResponseEntity.ok(user);
    }

    @PostMapping("/{id}/permissions")
    @PreAuthorize("hasRole('ADMIN') or hasPermission('user', 'update')")
    @Operation(summary = "Assign permissions", description = "Assign permissions to user")
    @Timed(value = "user.assign.permissions", description = "Time taken to assign permissions")
    public ResponseEntity<UserResponseDto> assignPermissions(
            @Parameter(description = "User ID") @PathVariable Long id,
            @Valid @RequestBody AssignPermissionsRequestDto request,
            Authentication authentication) {
        
        log.info("Assigning permissions {} to user with ID: {}", request.getPermissions(), id);
        UserResponseDto user = userService.assignPermissions(id, request.getPermissions(), authentication.getName());
        return ResponseEntity.ok(user);
    }

    @GetMapping("/{id}/sessions")
    @PreAuthorize("hasRole('ADMIN') or hasPermission('user', 'read') or #id == authentication.name")
    @Operation(summary = "Get user sessions", description = "Get active sessions for user")
    @Timed(value = "user.sessions", description = "Time taken to get user sessions")
    public ResponseEntity<List<UserSessionDto>> getUserSessions(
            @Parameter(description = "User ID") @PathVariable Long id) {
        
        log.info("Getting sessions for user with ID: {}", id);
        List<UserSessionDto> sessions = userService.getUserSessions(id);
        return ResponseEntity.ok(sessions);
    }

    @PostMapping("/{id}/sessions/{sessionId}/revoke")
    @PreAuthorize("hasRole('ADMIN') or hasPermission('user', 'update') or #id == authentication.name")
    @Operation(summary = "Revoke session", description = "Revoke a specific user session")
    @Timed(value = "user.session.revoke", description = "Time taken to revoke session")
    public ResponseEntity<Void> revokeSession(
            @Parameter(description = "User ID") @PathVariable Long id,
            @Parameter(description = "Session ID") @PathVariable String sessionId,
            Authentication authentication) {
        
        log.info("Revoking session {} for user with ID: {}", sessionId, id);
        userService.revokeSession(id, sessionId, authentication.getName());
        return ResponseEntity.noContent().build();
    }

    @PostMapping("/{id}/sessions/revoke-all")
    @PreAuthorize("hasRole('ADMIN') or hasPermission('user', 'update') or #id == authentication.name")
    @Operation(summary = "Revoke all sessions", description = "Revoke all sessions for user")
    @Timed(value = "user.sessions.revoke.all", description = "Time taken to revoke all sessions")
    public ResponseEntity<Void> revokeAllSessions(
            @Parameter(description = "User ID") @PathVariable Long id,
            Authentication authentication) {
        
        log.info("Revoking all sessions for user with ID: {}", id);
        userService.revokeAllSessions(id, authentication.getName());
        return ResponseEntity.noContent().build();
    }

    @GetMapping("/{id}/activities")
    @PreAuthorize("hasRole('ADMIN') or hasPermission('user', 'read') or #id == authentication.name")
    @Operation(summary = "Get user activities", description = "Get user activity log")
    @Timed(value = "user.activities", description = "Time taken to get user activities")
    public ResponseEntity<Page<UserActivityDto>> getUserActivities(
            @Parameter(description = "User ID") @PathVariable Long id,
            @Parameter(description = "Activity type filter") @RequestParam(required = false) String type,
            Pageable pageable) {
        
        log.info("Getting activities for user with ID: {}, type: {}", id, type);
        Page<UserActivityDto> activities = userService.getUserActivities(id, type, pageable);
        return ResponseEntity.ok(activities);
    }

    @PostMapping("/bulk")
    @PreAuthorize("hasRole('ADMIN') or hasPermission('user', 'create')")
    @Operation(summary = "Bulk create users", description = "Create multiple users")
    @Timed(value = "user.bulk.create", description = "Time taken to bulk create users")
    public ResponseEntity<BulkUserResponseDto> bulkCreateUsers(
            @Valid @RequestBody BulkCreateUsersRequestDto request,
            Authentication authentication) {
        
        log.info("Bulk creating {} users", request.getUsers().size());
        BulkUserResponseDto response = userService.bulkCreateUsers(request, authentication.getName());
        return ResponseEntity.status(HttpStatus.CREATED).body(response);
    }

    @GetMapping("/export")
    @PreAuthorize("hasRole('ADMIN') or hasPermission('user', 'export')")
    @Operation(summary = "Export users", description = "Export users to CSV")
    @Timed(value = "user.export", description = "Time taken to export users")
    public ResponseEntity<byte[]> exportUsers(
            @Parameter(description = "Export format") @RequestParam(defaultValue = "csv") String format,
            @Parameter(description = "Include inactive users") @RequestParam(defaultValue = "false") Boolean includeInactive) {
        
        log.info("Exporting users in format: {}, includeInactive: {}", format, includeInactive);
        byte[] exportData = userService.exportUsers(format, includeInactive);
        
        return ResponseEntity.ok()
                .header("Content-Disposition", "attachment; filename=users." + format)
                .header("Content-Type", "application/octet-stream")
                .body(exportData);
    }

    @GetMapping("/stats")
    @PreAuthorize("hasRole('ADMIN') or hasPermission('user', 'read')")
    @Operation(summary = "Get user statistics", description = "Get user statistics and metrics")
    @Timed(value = "user.stats", description = "Time taken to get user stats")
    public ResponseEntity<UserStatsDto> getUserStats() {
        log.info("Getting user statistics");
        UserStatsDto stats = userService.getUserStats();
        return ResponseEntity.ok(stats);
    }
}

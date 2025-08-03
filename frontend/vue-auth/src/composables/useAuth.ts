/**
 * Vue 3 Composition API for Authentication with Keycloak OIDC
 * Provides reactive authentication state and methods
 */

import { ref, reactive, computed, onMounted } from 'vue';
import Keycloak from 'keycloak-js';

// Types
interface User {
  id: string;
  username: string;
  email: string;
  firstName: string;
  lastName: string;
  roles: string[];
}

interface AuthState {
  isAuthenticated: boolean;
  user: User | null;
  token: string | null;
  loading: boolean;
  error: string | null;
}

// Keycloak configuration
const keycloakConfig = {
  url: import.meta.env.VITE_KEYCLOAK_URL || 'http://localhost:8080',
  realm: import.meta.env.VITE_KEYCLOAK_REALM || '002aic',
  clientId: import.meta.env.VITE_KEYCLOAK_CLIENT_ID || '002aic-frontend',
};

// Global state
const authState = reactive<AuthState>({
  isAuthenticated: false,
  user: null,
  token: null,
  loading: true,
  error: null,
});

// Keycloak instance
let keycloak: Keycloak | null = null;

// Composable function
export function useAuth() {
  // Initialize Keycloak
  const initAuth = async () => {
    try {
      authState.loading = true;
      authState.error = null;

      keycloak = new Keycloak(keycloakConfig);

      const authenticated = await keycloak.init({
        onLoad: 'check-sso',
        silentCheckSsoRedirectUri: window.location.origin + '/silent-check-sso.html',
        pkceMethod: 'S256',
      });

      if (authenticated) {
        authState.isAuthenticated = true;
        authState.token = keycloak.token || null;

        // Load user profile
        const profile = await keycloak.loadUserProfile();
        const roles = keycloak.realmAccess?.roles || [];

        authState.user = {
          id: keycloak.subject || '',
          username: profile.username || '',
          email: profile.email || '',
          firstName: profile.firstName || '',
          lastName: profile.lastName || '',
          roles: roles,
        };
      }

      // Set up token refresh
      keycloak.onTokenExpired = () => {
        keycloak?.updateToken(30).then((refreshed) => {
          if (refreshed) {
            authState.token = keycloak?.token || null;
          } else {
            console.warn('Token not refreshed, valid for another 30 seconds');
          }
        }).catch(() => {
          console.error('Failed to refresh token');
          logout();
        });
      };

    } catch (error) {
      console.error('Failed to initialize Keycloak:', error);
      authState.error = 'Authentication initialization failed';
    } finally {
      authState.loading = false;
    }
  };

  // Login function
  const login = async (): Promise<void> => {
    if (!keycloak) {
      throw new Error('Keycloak not initialized');
    }

    try {
      await keycloak.login({
        redirectUri: window.location.origin,
      });
    } catch (error) {
      console.error('Login failed:', error);
      authState.error = 'Login failed';
      throw error;
    }
  };

  // Logout function
  const logout = (): void => {
    if (keycloak) {
      keycloak.logout({
        redirectUri: window.location.origin,
      });
    }
    
    authState.isAuthenticated = false;
    authState.user = null;
    authState.token = null;
  };

  // Check if user has specific role
  const hasRole = (role: string): boolean => {
    return authState.user?.roles.includes(role) || false;
  };

  // Check if user has specific permission
  const hasPermission = async (resource: string, action: string): Promise<boolean> => {
    if (!authState.token || !authState.user) return false;

    try {
      const response = await fetch(`${import.meta.env.VITE_API_URL}/v1/authz/check`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${authState.token}`,
        },
        body: JSON.stringify({
          user_id: authState.user.id,
          resource,
          action,
        }),
      });

      if (response.ok) {
        const result = await response.json();
        return result.allowed;
      }
      return false;
    } catch (error) {
      console.error('Permission check failed:', error);
      return false;
    }
  };

  // Computed properties
  const isAdmin = computed(() => hasRole('admin'));
  const isDeveloper = computed(() => hasRole('developer'));
  const isDataScientist = computed(() => hasRole('data-scientist'));
  const userDisplayName = computed(() => {
    if (!authState.user) return '';
    return `${authState.user.firstName} ${authState.user.lastName}`.trim() || authState.user.username;
  });

  // HTTP client with automatic token injection
  const apiClient = {
    get: async (url: string, options: RequestInit = {}) => {
      return fetch(`${import.meta.env.VITE_API_URL}${url}`, {
        ...options,
        headers: {
          ...options.headers,
          'Authorization': `Bearer ${authState.token}`,
          'Content-Type': 'application/json',
        },
      });
    },
    post: async (url: string, data: any, options: RequestInit = {}) => {
      return fetch(`${import.meta.env.VITE_API_URL}${url}`, {
        method: 'POST',
        ...options,
        headers: {
          ...options.headers,
          'Authorization': `Bearer ${authState.token}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(data),
      });
    },
    put: async (url: string, data: any, options: RequestInit = {}) => {
      return fetch(`${import.meta.env.VITE_API_URL}${url}`, {
        method: 'PUT',
        ...options,
        headers: {
          ...options.headers,
          'Authorization': `Bearer ${authState.token}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(data),
      });
    },
    delete: async (url: string, options: RequestInit = {}) => {
      return fetch(`${import.meta.env.VITE_API_URL}${url}`, {
        method: 'DELETE',
        ...options,
        headers: {
          ...options.headers,
          'Authorization': `Bearer ${authState.token}`,
          'Content-Type': 'application/json',
        },
      });
    },
  };

  // Initialize on first use
  onMounted(() => {
    if (!keycloak) {
      initAuth();
    }
  });

  return {
    // State
    ...authState,
    
    // Computed
    isAdmin,
    isDeveloper,
    isDataScientist,
    userDisplayName,
    
    // Methods
    initAuth,
    login,
    logout,
    hasRole,
    hasPermission,
    apiClient,
  };
}

// Plugin for Vue app
export default {
  install(app: any) {
    app.config.globalProperties.$auth = useAuth();
  },
};

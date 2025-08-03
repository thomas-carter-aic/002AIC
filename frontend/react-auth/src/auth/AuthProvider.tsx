/**
 * React Authentication Provider using Keycloak OIDC
 * Provides authentication context for the entire application
 */

import React, { createContext, useContext, useEffect, useState, ReactNode } from 'react';
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

interface AuthContextType {
  isAuthenticated: boolean;
  user: User | null;
  token: string | null;
  login: () => Promise<void>;
  logout: () => void;
  hasRole: (role: string) => boolean;
  hasPermission: (resource: string, action: string) => Promise<boolean>;
  loading: boolean;
}

// Keycloak configuration
const keycloakConfig = {
  url: process.env.REACT_APP_KEYCLOAK_URL || 'http://localhost:8080',
  realm: process.env.REACT_APP_KEYCLOAK_REALM || '002aic',
  clientId: process.env.REACT_APP_KEYCLOAK_CLIENT_ID || '002aic-frontend',
};

// Create Keycloak instance
const keycloak = new Keycloak(keycloakConfig);

// Create Auth Context
const AuthContext = createContext<AuthContextType | undefined>(undefined);

// Auth Provider Props
interface AuthProviderProps {
  children: ReactNode;
}

// Auth Provider Component
export const AuthProvider: React.FC<AuthProviderProps> = ({ children }) => {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  // Initialize Keycloak
  useEffect(() => {
    const initKeycloak = async () => {
      try {
        const authenticated = await keycloak.init({
          onLoad: 'check-sso',
          silentCheckSsoRedirectUri: window.location.origin + '/silent-check-sso.html',
          pkceMethod: 'S256',
        });

        if (authenticated) {
          setIsAuthenticated(true);
          setToken(keycloak.token || null);
          
          // Load user profile
          const profile = await keycloak.loadUserProfile();
          const roles = keycloak.realmAccess?.roles || [];
          
          setUser({
            id: keycloak.subject || '',
            username: profile.username || '',
            email: profile.email || '',
            firstName: profile.firstName || '',
            lastName: profile.lastName || '',
            roles: roles,
          });
        }

        // Set up token refresh
        keycloak.onTokenExpired = () => {
          keycloak.updateToken(30).then((refreshed) => {
            if (refreshed) {
              setToken(keycloak.token || null);
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
      } finally {
        setLoading(false);
      }
    };

    initKeycloak();
  }, []);

  // Login function
  const login = async (): Promise<void> => {
    try {
      await keycloak.login({
        redirectUri: window.location.origin,
      });
    } catch (error) {
      console.error('Login failed:', error);
      throw error;
    }
  };

  // Logout function
  const logout = (): void => {
    keycloak.logout({
      redirectUri: window.location.origin,
    });
    setIsAuthenticated(false);
    setUser(null);
    setToken(null);
  };

  // Check if user has specific role
  const hasRole = (role: string): boolean => {
    return user?.roles.includes(role) || false;
  };

  // Check if user has specific permission
  const hasPermission = async (resource: string, action: string): Promise<boolean> => {
    if (!token) return false;

    try {
      const response = await fetch(`${process.env.REACT_APP_API_URL}/v1/authz/check`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({
          user_id: user?.id,
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

  const contextValue: AuthContextType = {
    isAuthenticated,
    user,
    token,
    login,
    logout,
    hasRole,
    hasPermission,
    loading,
  };

  return (
    <AuthContext.Provider value={contextValue}>
      {children}
    </AuthContext.Provider>
  );
};

// Custom hook to use auth context
export const useAuth = (): AuthContextType => {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};

// Higher-order component for protecting routes
export const withAuth = <P extends object>(
  Component: React.ComponentType<P>
): React.FC<P> => {
  return (props: P) => {
    const { isAuthenticated, loading, login } = useAuth();

    if (loading) {
      return <div>Loading...</div>;
    }

    if (!isAuthenticated) {
      return (
        <div className="auth-required">
          <h2>Authentication Required</h2>
          <p>Please log in to access this page.</p>
          <button onClick={login}>Log In</button>
        </div>
      );
    }

    return <Component {...props} />;
  };
};

// Component for role-based access control
interface RequireRoleProps {
  role: string;
  children: ReactNode;
  fallback?: ReactNode;
}

export const RequireRole: React.FC<RequireRoleProps> = ({ 
  role, 
  children, 
  fallback = <div>Access denied. Required role: {role}</div> 
}) => {
  const { hasRole } = useAuth();

  if (!hasRole(role)) {
    return <>{fallback}</>;
  }

  return <>{children}</>;
};

// Component for permission-based access control
interface RequirePermissionProps {
  resource: string;
  action: string;
  children: ReactNode;
  fallback?: ReactNode;
}

export const RequirePermission: React.FC<RequirePermissionProps> = ({ 
  resource, 
  action, 
  children, 
  fallback = <div>Access denied. Insufficient permissions.</div> 
}) => {
  const { hasPermission } = useAuth();
  const [allowed, setAllowed] = useState<boolean | null>(null);

  useEffect(() => {
    hasPermission(resource, action).then(setAllowed);
  }, [resource, action, hasPermission]);

  if (allowed === null) {
    return <div>Checking permissions...</div>;
  }

  if (!allowed) {
    return <>{fallback}</>;
  }

  return <>{children}</>;
};

// HTTP client with automatic token injection
export const createAuthenticatedClient = (baseURL: string) => {
  return {
    get: async (url: string, options: RequestInit = {}) => {
      const { token } = useAuth();
      return fetch(`${baseURL}${url}`, {
        ...options,
        headers: {
          ...options.headers,
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
      });
    },
    post: async (url: string, data: any, options: RequestInit = {}) => {
      const { token } = useAuth();
      return fetch(`${baseURL}${url}`, {
        method: 'POST',
        ...options,
        headers: {
          ...options.headers,
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(data),
      });
    },
    put: async (url: string, data: any, options: RequestInit = {}) => {
      const { token } = useAuth();
      return fetch(`${baseURL}${url}`, {
        method: 'PUT',
        ...options,
        headers: {
          ...options.headers,
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(data),
      });
    },
    delete: async (url: string, options: RequestInit = {}) => {
      const { token } = useAuth();
      return fetch(`${baseURL}${url}`, {
        method: 'DELETE',
        ...options,
        headers: {
          ...options.headers,
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
      });
    },
  };
};

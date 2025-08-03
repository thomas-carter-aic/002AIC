/**
 * Angular Authentication Service using Keycloak OIDC
 * Provides authentication state management and HTTP interceptors
 */

import { Injectable } from '@angular/core';
import { BehaviorSubject, Observable, from, of } from 'rxjs';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { Router } from '@angular/router';
import { environment } from '../../environments/environment';

declare const Keycloak: any;

// Types
export interface User {
  id: string;
  username: string;
  email: string;
  firstName: string;
  lastName: string;
  roles: string[];
}

export interface AuthState {
  isAuthenticated: boolean;
  user: User | null;
  token: string | null;
  loading: boolean;
}

@Injectable({
  providedIn: 'root'
})
export class AuthService {
  private keycloak: any;
  private authStateSubject = new BehaviorSubject<AuthState>({
    isAuthenticated: false,
    user: null,
    token: null,
    loading: true
  });

  public authState$ = this.authStateSubject.asObservable();

  constructor(
    private http: HttpClient,
    private router: Router
  ) {
    this.initKeycloak();
  }

  // Initialize Keycloak
  private async initKeycloak(): Promise<void> {
    try {
      this.keycloak = new (window as any).Keycloak({
        url: environment.keycloak.url,
        realm: environment.keycloak.realm,
        clientId: environment.keycloak.clientId,
      });

      const authenticated = await this.keycloak.init({
        onLoad: 'check-sso',
        silentCheckSsoRedirectUri: window.location.origin + '/assets/silent-check-sso.html',
        pkceMethod: 'S256',
      });

      if (authenticated) {
        await this.updateAuthState();
      }

      // Set up token refresh
      this.keycloak.onTokenExpired = () => {
        this.keycloak.updateToken(30).then((refreshed: boolean) => {
          if (refreshed) {
            this.updateAuthState();
          } else {
            console.warn('Token not refreshed, valid for another 30 seconds');
          }
        }).catch(() => {
          console.error('Failed to refresh token');
          this.logout();
        });
      };

    } catch (error) {
      console.error('Failed to initialize Keycloak:', error);
    } finally {
      this.authStateSubject.next({
        ...this.authStateSubject.value,
        loading: false
      });
    }
  }

  // Update authentication state
  private async updateAuthState(): Promise<void> {
    try {
      const profile = await this.keycloak.loadUserProfile();
      const roles = this.keycloak.realmAccess?.roles || [];

      const user: User = {
        id: this.keycloak.subject || '',
        username: profile.username || '',
        email: profile.email || '',
        firstName: profile.firstName || '',
        lastName: profile.lastName || '',
        roles: roles,
      };

      this.authStateSubject.next({
        isAuthenticated: true,
        user,
        token: this.keycloak.token,
        loading: false
      });
    } catch (error) {
      console.error('Failed to update auth state:', error);
    }
  }

  // Login
  public login(): Promise<void> {
    return this.keycloak.login({
      redirectUri: window.location.origin,
    });
  }

  // Logout
  public logout(): void {
    this.keycloak.logout({
      redirectUri: window.location.origin,
    });
    
    this.authStateSubject.next({
      isAuthenticated: false,
      user: null,
      token: null,
      loading: false
    });
  }

  // Get current user
  public getCurrentUser(): User | null {
    return this.authStateSubject.value.user;
  }

  // Get current token
  public getToken(): string | null {
    return this.authStateSubject.value.token;
  }

  // Check if authenticated
  public isAuthenticated(): boolean {
    return this.authStateSubject.value.isAuthenticated;
  }

  // Check if user has specific role
  public hasRole(role: string): boolean {
    const user = this.getCurrentUser();
    return user?.roles.includes(role) || false;
  }

  // Check if user has specific permission
  public hasPermission(resource: string, action: string): Observable<boolean> {
    const user = this.getCurrentUser();
    const token = this.getToken();

    if (!user || !token) {
      return of(false);
    }

    const headers = new HttpHeaders({
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json'
    });

    return this.http.post<{allowed: boolean}>(`${environment.apiUrl}/v1/authz/check`, {
      user_id: user.id,
      resource,
      action
    }, { headers }).pipe(
      map(response => response.allowed),
      catchError(error => {
        console.error('Permission check failed:', error);
        return of(false);
      })
    );
  }

  // Get HTTP headers with authentication
  public getAuthHeaders(): HttpHeaders {
    const token = this.getToken();
    return new HttpHeaders({
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json'
    });
  }

  // HTTP methods with automatic authentication
  public get<T>(url: string): Observable<T> {
    return this.http.get<T>(`${environment.apiUrl}${url}`, {
      headers: this.getAuthHeaders()
    });
  }

  public post<T>(url: string, data: any): Observable<T> {
    return this.http.post<T>(`${environment.apiUrl}${url}`, data, {
      headers: this.getAuthHeaders()
    });
  }

  public put<T>(url: string, data: any): Observable<T> {
    return this.http.put<T>(`${environment.apiUrl}${url}`, data, {
      headers: this.getAuthHeaders()
    });
  }

  public delete<T>(url: string): Observable<T> {
    return this.http.delete<T>(`${environment.apiUrl}${url}`, {
      headers: this.getAuthHeaders()
    });
  }
}

// HTTP Interceptor for automatic token injection
import { Injectable } from '@angular/core';
import { HttpInterceptor, HttpRequest, HttpHandler, HttpEvent } from '@angular/common/http';
import { Observable } from 'rxjs';

@Injectable()
export class AuthInterceptor implements HttpInterceptor {
  constructor(private authService: AuthService) {}

  intercept(req: HttpRequest<any>, next: HttpHandler): Observable<HttpEvent<any>> {
    const token = this.authService.getToken();
    
    if (token && req.url.startsWith(environment.apiUrl)) {
      const authReq = req.clone({
        headers: req.headers.set('Authorization', `Bearer ${token}`)
      });
      return next.handle(authReq);
    }
    
    return next.handle(req);
  }
}

// Auth Guard for protecting routes
import { Injectable } from '@angular/core';
import { CanActivate, ActivatedRouteSnapshot, RouterStateSnapshot, Router } from '@angular/router';
import { Observable, map, take } from 'rxjs';

@Injectable({
  providedIn: 'root'
})
export class AuthGuard implements CanActivate {
  constructor(
    private authService: AuthService,
    private router: Router
  ) {}

  canActivate(
    route: ActivatedRouteSnapshot,
    state: RouterStateSnapshot
  ): Observable<boolean> {
    return this.authService.authState$.pipe(
      take(1),
      map(authState => {
        if (authState.loading) {
          return false; // Still loading
        }
        
        if (!authState.isAuthenticated) {
          this.authService.login();
          return false;
        }
        
        // Check for required role
        const requiredRole = route.data['role'];
        if (requiredRole && !this.authService.hasRole(requiredRole)) {
          this.router.navigate(['/unauthorized']);
          return false;
        }
        
        return true;
      })
    );
  }
}

// Role Guard for role-based access control
@Injectable({
  providedIn: 'root'
})
export class RoleGuard implements CanActivate {
  constructor(
    private authService: AuthService,
    private router: Router
  ) {}

  canActivate(
    route: ActivatedRouteSnapshot,
    state: RouterStateSnapshot
  ): boolean {
    const requiredRole = route.data['role'];
    
    if (!requiredRole) {
      return true;
    }
    
    if (this.authService.hasRole(requiredRole)) {
      return true;
    }
    
    this.router.navigate(['/unauthorized']);
    return false;
  }
}

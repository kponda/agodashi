import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Router } from '@angular/router';
import { BehaviorSubject, Observable, tap, map, catchError, of } from 'rxjs';

// --- Interfaces ---
export interface UserResponse {
  id: number;
  username: string;
  email?: string;
  created_at: string; // ISO date string
}

export interface RegisterRequest {
  username: string;
  email?: string;
  password string; // Plain password
}

export interface LoginRequest {
  username: string;
  password string; // Plain password
}

export interface LoginResponse {
  access_token: string;
  user: UserResponse;
}

export interface RefreshTokenResponse {
  access_token: string;
}

@Injectable({
  providedIn: 'root'
})
export class AuthService {
  private authApiUrl = '/api/auth';
  private accessTokenKey = 'access_token';

  private isAuthenticatedSubject = new BehaviorSubject<boolean>(this.hasToken());
  private currentUserSubject = new BehaviorSubject<UserResponse | null>(this.getInitialUser());

  constructor(
    private http: HttpClient,
    private router: Router
  ) { }

  private hasToken(): boolean {
    return !!localStorage.getItem(this.accessTokenKey);
  }

  private getInitialUser(): UserResponse | null {
    // Optionally, try to load user from localStorage if you store it,
    // but typically user info is fetched after login or app init if token exists.
    // For now, start with null.
    return null;
  }

  register(userInfo: RegisterRequest): Observable<UserResponse> {
    return this.http.post<UserResponse>(`${this.authApiUrl}/register`, userInfo);
  }

  login(credentials: LoginRequest): Observable<LoginResponse> {
    return this.http.post<LoginResponse>(`${this.authApiUrl}/login`, credentials).pipe(
      tap(response => {
        if (response && response.access_token && response.user) {
          localStorage.setItem(this.accessTokenKey, response.access_token);
          this.isAuthenticatedSubject.next(true);
          this.currentUserSubject.next(response.user);
          this.router.navigate(['/articles']); // Navigate on successful login
        }
      }),
      catchError(error => {
        // Clear any partial login state if error occurs
        this.logoutSilently(); 
        throw error; // Re-throw the error to be caught by the component
      })
    );
  }

  logout(): void {
    // Optional: Call backend logout if it exists to invalidate refresh token on server-side
    // For now, client-side cleanup:
    localStorage.removeItem(this.accessTokenKey);
    this.isAuthenticatedSubject.next(false);
    this.currentUserSubject.next(null);
    this.router.navigate(['/auth/login']); // Navigate to login page
  }

  // Used internally on login error to prevent inconsistent state
  private logoutSilently(): void {
    localStorage.removeItem(this.accessTokenKey);
    this.isAuthenticatedSubject.next(false);
    this.currentUserSubject.next(null);
  }


  refreshToken(): Observable<RefreshTokenResponse> {
    return this.http.post<RefreshTokenResponse>(`${this.authApiUrl}/refresh`, {}).pipe(
      tap(response => {
        if (response && response.access_token) {
          localStorage.setItem(this.accessTokenKey, response.access_token);
          this.isAuthenticatedSubject.next(true); // User remains authenticated
          // Optionally, re-fetch user details if they might change or if not already loaded
        }
      }),
      catchError(error => {
        this.logout(); // If refresh fails, logout the user
        return of({ access_token: '' }); // Return empty observable or handle error as needed
      })
    );
  }

  getAccessToken(): string | null {
    return localStorage.getItem(this.accessTokenKey);
  }

  isAuthenticated$(): Observable<boolean> {
    return this.isAuthenticatedSubject.asObservable();
  }

  getCurrentUser$(): Observable<UserResponse | null> {
    return this.currentUserSubject.asObservable();
  }

  // Helper to load current user if authenticated but user subject is null
  // This might be called at app initialization or when AuthGuard allows access
  loadCurrentUser(): Observable<UserResponse | null> {
    if (this.hasToken() && !this.currentUserSubject.value) {
      // This endpoint doesn't exist yet, but illustrates the concept.
      // Typically, you'd have a '/api/auth/me' or '/api/users/me' endpoint.
      // For now, we can't implement this without a backend endpoint.
      // If user data is part of refresh token or access token (not ideal for access token),
      // it could be decoded here. Or, store user on login and retrieve from local storage.
      console.warn('loadCurrentUser: "me" endpoint not implemented. User details not loaded on init.');
      return of(null); 
    }
    return of(this.currentUserSubject.value);
  }
}

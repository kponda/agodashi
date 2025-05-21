import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common'; // For *ngIf, *ngFor, etc.
import { ReactiveFormsModule, FormBuilder, FormGroup, Validators } from '@angular/forms';
import { Router, RouterModule } from '@angular/router';
import { AuthService, LoginRequest } from '../../services/auth.service'; // Import AuthService and LoginRequest

@Component({
  selector: 'app-login',
  standalone: true,
  imports: [
    CommonModule,
    ReactiveFormsModule,
    RouterModule // For routerLink if needed, or for Router injection
  ],
  templateUrl: './login.component.html',
  styleUrls: ['./login.component.scss']
})
export class LoginComponent implements OnInit {
  loginForm!: FormGroup; // Definite assignment assertion
  isSubmitting = false;
  errorMessage: string | null = null;

  constructor(
    private fb: FormBuilder,
    private router: Router, // Router might still be needed for other navigations if any
    private authService: AuthService // Inject AuthService
  ) { }

  ngOnInit(): void {
    this.loginForm = this.fb.group({
      username: ['', [Validators.required]],
      password: ['', [Validators.required]]
    });
  }

  onSubmit(): void {
    if (this.loginForm.invalid) {
      this.markFormGroupTouched(this.loginForm);
      this.errorMessage = 'Please fill in all fields.';
      return;
    }

    this.isSubmitting = true;
    this.errorMessage = null;
    const { username, password } = this.loginForm.value;

    const credentials: LoginRequest = { username, password };

    this.authService.login(credentials).subscribe({
      next: (response) => {
        // Navigation is handled by AuthService on successful login
        this.isSubmitting = false;
        // Optionally, can show a success message briefly or rely on navigation
        console.log('Login successful, user:', response.user);
      },
      error: (err) => {
        this.isSubmitting = false;
        if (err.status === 401) {
          this.errorMessage = 'Invalid username or password.';
        } else if (err.error && typeof err.error === 'string' && err.error.includes("User not found")) { // Example more specific error
            this.errorMessage = 'Invalid username or password.';
        } else if (err.error && err.error.message) {
            this.errorMessage = err.error.message;
        }
        else {
          this.errorMessage = 'An unexpected error occurred during login. Please try again.';
        }
        console.error('Login error:', err);
      }
    });
  }

  // Helper to mark all fields in a form group as touched
  private markFormGroupTouched(formGroup: FormGroup) {
    Object.values(formGroup.controls).forEach(control => {
      control.markAsTouched();
      if (control instanceof FormGroup) {
        this.markFormGroupTouched(control);
      }
    });
  }

  // Convenience getters for template
  get username() { return this.loginForm.get('username'); }
  get password() { return this.loginForm.get('password'); }
}

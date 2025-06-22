import { Component } from '@angular/core';
import { RouterOutlet, RouterLink, RouterLinkActive } from '@angular/router'; // Import RouterLink and RouterLinkActive
import { CommonModule } from '@angular/common'; // Import CommonModule for *ngIf, etc.

@Component({
  selector: 'app-root',
  // standalone: true, // Ensure this is present if your project is setup for standalone components
  imports: [CommonModule, RouterOutlet, RouterLink, RouterLinkActive], // Add CommonModule, RouterLink, RouterLinkActive
  templateUrl: './app.component.html',
  styleUrl: './app.component.scss'
})
export class AppComponent {
  title = 'app';
  currentYear = new Date().getFullYear(); // For the footer

  // Placeholder for loading state, manage this via a service in a real app
  isLoading: boolean = false;

  // Placeholder for logout logic, manage this via AuthService
  logout(): void {
    console.log('Logout action triggered');
    // Call your AuthService logout method here
  }
}

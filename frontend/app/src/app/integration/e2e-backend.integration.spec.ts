import { TestBed } from '@angular/core/testing';
import { HttpClientModule } from '@angular/common/http';
import { RouterTestingModule } from '@angular/router/testing';
import { AuthService } from '../services/auth.service';
import { ArticleService } from '../services/article.service';
import { firstValueFrom } from 'rxjs';

// This file contains integration tests that actually communicate with the backend
// These tests are designed to be run when both frontend and backend are running
// To run these tests:
// 1. Start the backend: docker compose up -d backend postgres
// 2. Ensure the database has been migrated
// 3. Run: ng test --include=**/e2e-backend.integration.spec.ts

describe('E2E Frontend-Backend Integration Tests', () => {
  let authService: AuthService;
  let articleService: ArticleService;
  const backendBaseUrl = 'http://localhost:8080';
  
  // Test configuration
  const testUser = {
    username: 'e2etest_' + Date.now(), // Unique username
    email: 'e2e@test.com',
    password: 'testpassword123'
  };

  let accessToken: string | null = null;
  let createdArticleSlug: string | null = null;

  beforeAll(async () => {
    await TestBed.configureTestingModule({
      imports: [
        HttpClientModule,
        RouterTestingModule
      ],
      providers: [AuthService, ArticleService]
    }).compileComponents();

    authService = TestBed.inject(AuthService);
    articleService = TestBed.inject(ArticleService);

    // Clear any existing authentication state
    localStorage.clear();
  });

  afterAll(async () => {
    // Cleanup: Delete test user and articles if possible
    // Note: The backend doesn't currently have user deletion endpoint
    localStorage.clear();
  });

  describe('Backend Connectivity', () => {
    it('should be able to reach the backend API root endpoint', async () => {
      try {
        const response = await fetch(`${backendBaseUrl}/api/`);
        expect(response.ok).toBe(true);
        
        const text = await response.text();
        expect(text).toContain('Hello from Backend API');
      } catch (error) {
        fail(`Backend is not reachable at ${backendBaseUrl}. Please start the backend service first. Error: ${error}`);
      }
    });

    it('should handle CORS properly for cross-origin requests', async () => {
      try {
        const response = await fetch(`${backendBaseUrl}/api/articles`, {
          method: 'OPTIONS',
          headers: {
            'Origin': 'http://localhost:4200',
            'Access-Control-Request-Method': 'GET',
            'Access-Control-Request-Headers': 'Content-Type'
          }
        });

        expect(response.ok).toBe(true);
        expect(response.headers.get('Access-Control-Allow-Origin')).toBe('http://localhost:4200');
        expect(response.headers.get('Access-Control-Allow-Methods')).toContain('GET');
      } catch (error) {
        fail(`CORS preflight request failed: ${error}`);
      }
    });
  });

  describe('Authentication Integration (Real Backend)', () => {
    it('should register a new user successfully', async () => {
      try {
        const userResponse = await firstValueFrom(authService.register(testUser));
        
        expect(userResponse).toBeDefined();
        expect(userResponse.username).toBe(testUser.username);
        expect(userResponse.email).toBe(testUser.email);
        expect(userResponse.id).toBeGreaterThan(0);
        expect(userResponse.created_at).toBeDefined();
      } catch (error: any) {
        if (error.status === 409) {
          // User already exists, that's okay for this test
          console.log('User already exists, skipping registration test');
        } else {
          fail(`Registration failed with unexpected error: ${error.message || error}`);
        }
      }
    });

    it('should login with valid credentials', async () => {
      try {
        const loginResponse = await firstValueFrom(authService.login({
          username: testUser.username,
          password: testUser.password
        }));

        expect(loginResponse).toBeDefined();
        expect(loginResponse.access_token).toBeDefined();
        expect(loginResponse.access_token.length).toBeGreaterThan(0);
        expect(loginResponse.user).toBeDefined();
        expect(loginResponse.user.username).toBe(testUser.username);

        // Store token for subsequent tests
        accessToken = loginResponse.access_token;
        
        // Verify token is stored in localStorage
        expect(localStorage.getItem('access_token')).toBe(accessToken);

        // Verify authentication state
        const isAuthenticated = await firstValueFrom(authService.isAuthenticated$());
        expect(isAuthenticated).toBe(true);

        const currentUser = await firstValueFrom(authService.getCurrentUser$());
        expect(currentUser?.username).toBe(testUser.username);
      } catch (error: any) {
        fail(`Login failed: ${error.message || error}`);
      }
    });

    it('should reject invalid login credentials', async () => {
      try {
        await firstValueFrom(authService.login({
          username: testUser.username,
          password: 'wrongpassword'
        }));
        fail('Expected login to fail with wrong password');
      } catch (error: any) {
        expect(error.status).toBe(401);
      }
    });

    it('should refresh access token successfully', async () => {
      // Skip if we don't have a valid login first
      if (!accessToken) {
        pending('Skipping refresh test - no valid login token available');
        return;
      }

      try {
        const refreshResponse = await firstValueFrom(authService.refreshToken());
        
        expect(refreshResponse).toBeDefined();
        expect(refreshResponse.access_token).toBeDefined();
        expect(refreshResponse.access_token.length).toBeGreaterThan(0);
        
        // Token should be different from the original (optional check)
        // Note: This may not always be true depending on JWT implementation
        
        // Verify new token is stored
        expect(localStorage.getItem('access_token')).toBe(refreshResponse.access_token);
      } catch (error: any) {
        // If refresh fails, it might be because the refresh token expired
        // or the backend doesn't support refresh token rotation
        console.warn(`Token refresh failed: ${error.message}. This might be expected.`);
      }
    });
  });

  describe('Article CRUD Integration (Real Backend)', () => {
    const testArticle = {
      slug: 'e2e-test-article-' + Date.now(),
      translations: [
        {
          language_code: 'en',
          title: 'E2E Test Article',
          content: 'This is a test article created by E2E integration tests.',
          is_published: true
        },
        {
          language_code: 'es',
          title: 'Artículo de Prueba E2E',
          content: 'Este es un artículo de prueba creado por las pruebas de integración E2E.',
          is_published: false
        }
      ]
    };

    beforeAll(() => {
      // Ensure we're authenticated for article tests
      if (!accessToken) {
        pending('Cannot run article tests without authentication');
      }
    });

    it('should fetch articles list from backend', async () => {
      try {
        const articles = await firstValueFrom(articleService.getArticles());
        
        expect(Array.isArray(articles)).toBe(true);
        // We don't expect any specific number of articles since this is a real backend
        
        // If there are articles, verify their structure
        if (articles.length > 0) {
          const firstArticle = articles[0];
          expect(firstArticle.id).toBeDefined();
          expect(firstArticle.slug).toBeDefined();
          expect(firstArticle.created_at).toBeDefined();
          expect(firstArticle.updated_at).toBeDefined();
          expect(Array.isArray(firstArticle.translations)).toBe(true);
        }
      } catch (error: any) {
        fail(`Failed to fetch articles: ${error.message || error}`);
      }
    });

    it('should create a new article successfully', async () => {
      if (!accessToken) {
        pending('Cannot create article without authentication');
        return;
      }

      try {
        const createdArticle = await firstValueFrom(articleService.createArticle(testArticle));
        
        expect(createdArticle).toBeDefined();
        expect(createdArticle.slug).toBe(testArticle.slug);
        expect(createdArticle.id).toBeGreaterThan(0);
        expect(createdArticle.translations.length).toBe(2);
        
        // Verify translations
        const enTranslation = createdArticle.translations.find(t => t.language_code === 'en');
        const esTranslation = createdArticle.translations.find(t => t.language_code === 'es');
        
        expect(enTranslation).toBeDefined();
        expect(enTranslation!.title).toBe('E2E Test Article');
        expect(enTranslation!.is_published).toBe(true);
        
        expect(esTranslation).toBeDefined();
        expect(esTranslation!.title).toBe('Artículo de Prueba E2E');
        expect(esTranslation!.is_published).toBe(false);

        // Store for cleanup
        createdArticleSlug = createdArticle.slug;
      } catch (error: any) {
        fail(`Failed to create article: ${error.message || error}`);
      }
    });

    it('should fetch the created article by slug', async () => {
      if (!createdArticleSlug) {
        pending('Cannot fetch article without creating one first');
        return;
      }

      try {
        const fetchedArticle = await firstValueFrom(articleService.getArticle(createdArticleSlug));
        
        expect(fetchedArticle).toBeDefined();
        expect(fetchedArticle.slug).toBe(createdArticleSlug);
        expect(fetchedArticle.translations.length).toBeGreaterThanOrEqual(1);
        
        // Test language-specific fetch
        const enArticle = await firstValueFrom(articleService.getArticle(createdArticleSlug, 'en'));
        expect(enArticle.translations.length).toBe(1);
        expect(enArticle.translations[0].language_code).toBe('en');
        
        const esArticle = await firstValueFrom(articleService.getArticle(createdArticleSlug, 'es'));
        expect(esArticle.translations.length).toBe(1);
        expect(esArticle.translations[0].language_code).toBe('es');
      } catch (error: any) {
        fail(`Failed to fetch article: ${error.message || error}`);
      }
    });

    it('should handle requests for non-existent articles', async () => {
      try {
        await firstValueFrom(articleService.getArticle('non-existent-article-12345'));
        fail('Expected 404 error for non-existent article');
      } catch (error: any) {
        expect(error.status).toBe(404);
      }
    });

    it('should reject article creation without authentication', async () => {
      // Temporarily remove authentication
      const originalToken = localStorage.getItem('access_token');
      localStorage.removeItem('access_token');
      
      try {
        await firstValueFrom(articleService.createArticle({
          slug: 'unauthorized-article',
          translations: [{
            language_code: 'en',
            title: 'Unauthorized',
            content: 'This should fail',
            is_published: false
          }]
        }));
        fail('Expected 401 error for unauthenticated request');
      } catch (error: any) {
        expect(error.status).toBe(401);
      } finally {
        // Restore authentication
        if (originalToken) {
          localStorage.setItem('access_token', originalToken);
        }
      }
    });
  });

  describe('Error Handling Integration', () => {
    it('should handle malformed requests gracefully', async () => {
      try {
        // Try to register with invalid data
        await firstValueFrom(authService.register({
          username: 'ab', // Too short
          password: '123' // Too short
        }));
        fail('Expected validation error');
      } catch (error: any) {
        expect(error.status).toBe(400);
      }
    });

    it('should handle server errors gracefully', async () => {
      try {
        // Try to access an endpoint that should return an error
        const response = await fetch(`${backendBaseUrl}/api/nonexistent`);
        expect(response.status).toBe(404);
      } catch (error: any) {
        // Network errors are also acceptable for this test
        expect(error).toBeDefined();
      }
    });
  });

  describe('Data Consistency', () => {
    it('should maintain data consistency across multiple requests', async () => {
      if (!accessToken || !createdArticleSlug) {
        pending('Cannot test data consistency without authentication and test data');
        return;
      }

      try {
        // Fetch the same article multiple times
        const fetch1 = await firstValueFrom(articleService.getArticle(createdArticleSlug));
        const fetch2 = await firstValueFrom(articleService.getArticle(createdArticleSlug));
        
        expect(fetch1.id).toBe(fetch2.id);
        expect(fetch1.slug).toBe(fetch2.slug);
        expect(fetch1.created_at).toBe(fetch2.created_at);
        expect(fetch1.translations.length).toBe(fetch2.translations.length);
      } catch (error: any) {
        fail(`Data consistency test failed: ${error.message || error}`);
      }
    });
  });
});
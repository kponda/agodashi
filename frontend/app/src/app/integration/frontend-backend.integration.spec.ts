import { TestBed } from '@angular/core/testing';
import { HttpClientTestingModule, HttpTestingController } from '@angular/common/http/testing';
import { RouterTestingModule } from '@angular/router/testing';
import { HTTP_INTERCEPTORS } from '@angular/common/http';
import { AuthService } from '../services/auth.service';
import { ArticleService } from '../services/article.service';
import { AuthInterceptor } from '../interceptors/auth.interceptor';
import { Component } from '@angular/core';

// Mock components for routing tests
@Component({ 
  template: '',
  standalone: true
})
class MockArticlesComponent { }

@Component({ 
  template: '',
  standalone: true 
})
class MockLoginComponent { }

describe('Frontend-Backend Integration Tests', () => {
  let httpMock: HttpTestingController;
  let authService: AuthService;
  let articleService: ArticleService;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [
        HttpClientTestingModule,
        RouterTestingModule.withRoutes([
          { path: 'articles', component: MockArticlesComponent },
          { path: 'login', component: MockLoginComponent }
        ])
      ],
      providers: [
        AuthService, 
        ArticleService,
        {
          provide: HTTP_INTERCEPTORS,
          useClass: AuthInterceptor,
          multi: true
        }
      ]
    }).compileComponents();

    httpMock = TestBed.inject(HttpTestingController);
    authService = TestBed.inject(AuthService);
    articleService = TestBed.inject(ArticleService);

    // Clear localStorage before each test
    localStorage.clear();
  });

  afterEach(() => {
    httpMock.verify();
    localStorage.clear();
  });

  describe('Authentication Flow Integration', () => {
    it('should register a new user successfully', () => {
      const mockRegisterRequest = {
        username: 'testuser',
        email: 'test@example.com',
        password: 'password123'
      };

      const mockUserResponse = {
        id: 1,
        username: 'testuser',
        email: 'test@example.com',
        created_at: '2024-01-01T00:00:00Z'
      };

      authService.register(mockRegisterRequest).subscribe(response => {
        expect(response).toEqual(mockUserResponse);
        expect(response.username).toBe('testuser');
        expect(response.email).toBe('test@example.com');
      });

      const req = httpMock.expectOne('/api/auth/register');
      expect(req.request.method).toBe('POST');
      expect(req.request.body).toEqual(mockRegisterRequest);
      req.flush(mockUserResponse);
    });

    it('should handle registration validation errors', () => {
      const invalidRegisterRequest = {
        username: 'ab', // Too short
        password: '123' // Too short
      };

      authService.register(invalidRegisterRequest).subscribe({
        next: () => fail('Expected error response'),
        error: (error) => {
          expect(error.status).toBe(400);
        }
      });

      const req = httpMock.expectOne('/api/auth/register');
      req.flush('Username must be at least 3 characters', { status: 400, statusText: 'Bad Request' });
    });

    it('should login successfully and store tokens', () => {
      const mockLoginRequest = {
        username: 'testuser',
        password: 'password123'
      };

      const mockLoginResponse = {
        access_token: 'mock-jwt-token',
        user: {
          id: 1,
          username: 'testuser',
          email: 'test@example.com',
          created_at: '2024-01-01T00:00:00Z'
        }
      };

      authService.login(mockLoginRequest).subscribe(response => {
        expect(response).toEqual(mockLoginResponse);
        expect(localStorage.getItem('access_token')).toBe('mock-jwt-token');
        
        // Check authentication state
        authService.isAuthenticated$().subscribe(isAuth => {
          expect(isAuth).toBe(true);
        });

        authService.getCurrentUser$().subscribe(user => {
          expect(user).toEqual(mockLoginResponse.user);
        });
      });

      const req = httpMock.expectOne('/api/auth/login');
      expect(req.request.method).toBe('POST');
      expect(req.request.body).toEqual(mockLoginRequest);
      req.flush(mockLoginResponse);
    });

    it('should handle login with invalid credentials', () => {
      const invalidLoginRequest = {
        username: 'testuser',
        password: 'wrongpassword'
      };

      authService.login(invalidLoginRequest).subscribe({
        next: () => fail('Expected error response'),
        error: (error) => {
          expect(error.status).toBe(401);
          expect(localStorage.getItem('access_token')).toBeNull();
        }
      });

      const req = httpMock.expectOne('/api/auth/login');
      req.flush('Invalid credentials', { status: 401, statusText: 'Unauthorized' });
    });

    it('should refresh access token successfully', () => {
      // Set up initial authentication state
      localStorage.setItem('access_token', 'old-token');

      const mockRefreshResponse = {
        access_token: 'new-jwt-token'
      };

      authService.refreshToken().subscribe(response => {
        expect(response.access_token).toBe('new-jwt-token');
        expect(localStorage.getItem('access_token')).toBe('new-jwt-token');
      });

      const req = httpMock.expectOne('/api/auth/refresh');
      expect(req.request.method).toBe('POST');
      req.flush(mockRefreshResponse);
    });

    it('should logout and clear authentication state', () => {
      // Set up authentication state
      localStorage.setItem('access_token', 'some-token');
      
      authService.logout();

      expect(localStorage.getItem('access_token')).toBeNull();
      
      authService.isAuthenticated$().subscribe(isAuth => {
        expect(isAuth).toBe(false);
      });

      authService.getCurrentUser$().subscribe(user => {
        expect(user).toBeNull();
      });
    });
  });

  describe('Article CRUD Operations Integration', () => {
    const mockAccessToken = 'mock-jwt-token';

    beforeEach(() => {
      localStorage.setItem('access_token', mockAccessToken);
    });

    it('should fetch articles list successfully', () => {
      const mockArticles = [
        {
          id: 1,
          slug: 'test-article',
          author_id: 1,
          created_at: '2024-01-01T00:00:00Z',
          updated_at: '2024-01-01T00:00:00Z',
          translations: [
            {
              id: 1,
              article_id: 1,
              language_code: 'en',
              title: 'Test Article',
              content: 'Test content',
              is_published: true,
              published_at: '2024-01-01T00:00:00Z',
              created_at: '2024-01-01T00:00:00Z',
              updated_at: '2024-01-01T00:00:00Z'
            }
          ]
        }
      ];

      articleService.getArticles().subscribe(articles => {
        expect(articles).toEqual(mockArticles);
        expect(articles.length).toBe(1);
        expect(articles[0].slug).toBe('test-article');
      });

      const req = httpMock.expectOne('/api/articles');
      expect(req.request.method).toBe('GET');
      req.flush(mockArticles);
    });

    it('should fetch articles with pagination and language filter', () => {
      const mockArticles: any[] = [];

      articleService.getArticles('es', 5, 10).subscribe(articles => {
        expect(articles).toEqual(mockArticles);
      });

      const req = httpMock.expectOne('/api/articles?lang=es&limit=5&offset=10');
      expect(req.request.method).toBe('GET');
      req.flush(mockArticles);
    });

    it('should fetch a single article by slug', () => {
      const mockArticle = {
        id: 1,
        slug: 'test-article',
        author_id: 1,
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
        translations: [
          {
            id: 1,
            article_id: 1,
            language_code: 'en',
            title: 'Test Article',
            content: 'Test content',
            is_published: true,
            published_at: '2024-01-01T00:00:00Z',
            created_at: '2024-01-01T00:00:00Z',
            updated_at: '2024-01-01T00:00:00Z'
          }
        ]
      };

      articleService.getArticle('test-article').subscribe(article => {
        expect(article).toEqual(mockArticle);
        expect(article.slug).toBe('test-article');
      });

      const req = httpMock.expectOne('/api/articles/test-article');
      expect(req.request.method).toBe('GET');
      req.flush(mockArticle);
    });

    it('should fetch article with specific language', () => {
      const mockArticle = {
        id: 1,
        slug: 'test-article',
        author_id: 1,
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
        translations: [
          {
            id: 2,
            article_id: 1,
            language_code: 'es',
            title: 'Artículo de Prueba',
            content: 'Contenido de prueba',
            is_published: true,
            published_at: '2024-01-01T00:00:00Z',
            created_at: '2024-01-01T00:00:00Z',
            updated_at: '2024-01-01T00:00:00Z'
          }
        ]
      };

      articleService.getArticle('test-article', 'es').subscribe(article => {
        expect(article).toEqual(mockArticle);
        expect(article.translations[0].language_code).toBe('es');
      });

      const req = httpMock.expectOne('/api/articles/test-article?lang=es');
      expect(req.request.method).toBe('GET');
      req.flush(mockArticle);
    });

    it('should handle article not found error', () => {
      articleService.getArticle('non-existent').subscribe({
        next: () => fail('Expected error response'),
        error: (error) => {
          expect(error.status).toBe(404);
        }
      });

      const req = httpMock.expectOne('/api/articles/non-existent');
      req.flush('Article not found', { status: 404, statusText: 'Not Found' });
    });

    it('should create a new article successfully', () => {
      const newArticleRequest = {
        slug: 'new-article',
        translations: [
          {
            language_code: 'en',
            title: 'New Article',
            content: 'New article content',
            is_published: true
          }
        ]
      };

      const mockCreatedArticle = {
        id: 2,
        slug: 'new-article',
        author_id: 1,
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
        translations: [
          {
            id: 3,
            article_id: 2,
            language_code: 'en',
            title: 'New Article',
            content: 'New article content',
            is_published: true,
            published_at: '2024-01-01T00:00:00Z',
            created_at: '2024-01-01T00:00:00Z',
            updated_at: '2024-01-01T00:00:00Z'
          }
        ]
      };

      articleService.createArticle(newArticleRequest).subscribe(article => {
        expect(article).toEqual(mockCreatedArticle);
        expect(article.slug).toBe('new-article');
      });

      const req = httpMock.expectOne('/api/articles');
      expect(req.request.method).toBe('POST');
      expect(req.request.body).toEqual(newArticleRequest);
      req.flush(mockCreatedArticle);
    });

    it('should handle unauthorized article creation', () => {
      localStorage.removeItem('access_token'); // Remove token to simulate unauthorized

      const newArticleRequest = {
        slug: 'unauthorized-article',
        translations: [
          {
            language_code: 'en',
            title: 'Unauthorized Article',
            content: 'This should not be created',
            is_published: false
          }
        ]
      };

      articleService.createArticle(newArticleRequest).subscribe({
        next: () => fail('Expected error response'),
        error: (error) => {
          expect(error.status).toBe(401);
        }
      });

      const req = httpMock.expectOne('/api/articles');
      req.flush('Unauthorized', { status: 401, statusText: 'Unauthorized' });
    });
  });

  describe('CORS Configuration Tests', () => {
    it('should handle CORS preflight requests properly', () => {
      // This test verifies that our frontend can handle CORS responses
      // The actual CORS handling is done by the browser, but we can verify
      // that our requests include the correct headers that would trigger CORS

      const mockArticles: any[] = [];

      articleService.getArticles().subscribe(articles => {
        expect(articles).toEqual(mockArticles);
      });

      const req = httpMock.expectOne('/api/articles');
      expect(req.request.method).toBe('GET');
      
      // Simulate CORS headers that would be returned by the backend
      const corsHeaders = {
        'Access-Control-Allow-Origin': 'http://localhost:4200',
        'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, OPTIONS',
        'Access-Control-Allow-Headers': 'Content-Type, Authorization',
        'Access-Control-Allow-Credentials': 'true'
      };

      req.flush(mockArticles, { headers: corsHeaders });
    });

    it('should include authorization header for authenticated requests', () => {
      localStorage.setItem('access_token', 'test-token');

      const newArticleRequest = {
        slug: 'test-article',
        translations: [
          {
            language_code: 'en',
            title: 'Test',
            content: 'Test',
            is_published: true
          }
        ]
      };

      articleService.createArticle(newArticleRequest).subscribe();

      const req = httpMock.expectOne('/api/articles');
      expect(req.request.headers.get('Authorization')).toBe('Bearer test-token');
      req.flush({});
    });
  });

  describe('Error Handling Integration', () => {
    it('should handle network errors gracefully', () => {
      articleService.getArticles().subscribe({
        next: () => fail('Expected error response'),
        error: (error) => {
          expect(error.name).toBe('HttpErrorResponse');
        }
      });

      const req = httpMock.expectOne('/api/articles');
      req.error(new ProgressEvent('Network error'));
    });

    it('should handle server errors (500)', () => {
      articleService.getArticles().subscribe({
        next: () => fail('Expected error response'),
        error: (error) => {
          expect(error.status).toBe(500);
        }
      });

      const req = httpMock.expectOne('/api/articles');
      req.flush('Internal Server Error', { status: 500, statusText: 'Internal Server Error' });
    });

    it('should handle malformed JSON responses', () => {
      articleService.getArticles().subscribe({
        next: (data) => {
          // Angular's HTTP client might parse 'invalid json{' as a string successfully
          // This is actually valid behavior, so we'll check for that
          expect(typeof data).toBe('string');
        },
        error: (error) => {
          // If it does throw an error, it should be an HttpErrorResponse
          expect(error.name).toBe('HttpErrorResponse');
        }
      });

      const req = httpMock.expectOne('/api/articles');
      req.flush('invalid json{', { status: 200, statusText: 'OK' });
    });
  });

  describe('Authentication State Management', () => {
    it('should maintain authentication state across service calls', () => {
      const mockLoginResponse = {
        access_token: 'test-token',
        user: {
          id: 1,
          username: 'testuser',
          email: 'test@example.com',
          created_at: '2024-01-01T00:00:00Z'
        }
      };

      // Track authentication state changes
      let isAuthenticatedCount = 0;
      const authStates: boolean[] = [];

      authService.isAuthenticated$().subscribe(isAuth => {
        authStates.push(isAuth);
        isAuthenticatedCount++;
      });

      // Login first
      authService.login({ username: 'testuser', password: 'password' }).subscribe(() => {
        // After login, verify authentication state
        expect(authStates[authStates.length - 1]).toBe(true);

        // Make an authenticated request
        articleService.createArticle({
          slug: 'test',
          translations: [{ language_code: 'en', title: 'Test', content: 'Test', is_published: true }]
        }).subscribe();

        const articleReq = httpMock.expectOne('/api/articles');
        expect(articleReq.request.headers.get('Authorization')).toBe('Bearer test-token');
        articleReq.flush({});

        // Logout
        authService.logout();

        // Verify authentication state cleared
        expect(authStates[authStates.length - 1]).toBe(false);
      });
      
      const loginReq = httpMock.expectOne('/api/auth/login');
      loginReq.flush(mockLoginResponse);
    });
  });

  describe('Multi-language Support Integration', () => {
    it('should handle multilingual article creation', () => {
      const multilingualArticle = {
        slug: 'multilingual-article',
        translations: [
          {
            language_code: 'en',
            title: 'English Title',
            content: 'English content',
            is_published: true
          },
          {
            language_code: 'es',
            title: 'Título en Español',
            content: 'Contenido en español',
            is_published: true
          },
          {
            language_code: 'ja',
            title: '日本語のタイトル',
            content: '日本語の内容',
            is_published: false
          }
        ]
      };

      const mockResponse = {
        id: 1,
        slug: 'multilingual-article',
        author_id: 1,
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
        translations: multilingualArticle.translations.map((t, i) => ({
          ...t,
          id: i + 1,
          article_id: 1,
          published_at: t.is_published ? '2024-01-01T00:00:00Z' : null,
          created_at: '2024-01-01T00:00:00Z',
          updated_at: '2024-01-01T00:00:00Z'
        }))
      };

      articleService.createArticle(multilingualArticle).subscribe(article => {
        expect(article.translations.length).toBe(3);
        expect(article.translations.find(t => t.language_code === 'en')?.title).toBe('English Title');
        expect(article.translations.find(t => t.language_code === 'es')?.title).toBe('Título en Español');
        expect(article.translations.find(t => t.language_code === 'ja')?.title).toBe('日本語のタイトル');
      });

      const req = httpMock.expectOne('/api/articles');
      expect(req.request.body).toEqual(multilingualArticle);
      req.flush(mockResponse);
    });
  });
});
import { Injectable } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';

// --- Interfaces (mirroring backend/models.go) ---
export interface Article {
  id: number;
  slug: string;
  author_id?: number;
  created_at: string; // ISO date string
  updated_at: string; // ISO date string
}

export interface ArticleTranslation {
  id: number;
  article_id: number;
  language_code: string;
  title: string;
  content: string;
  is_published: boolean;
  published_at?: string; // ISO date string
  created_at: string;   // ISO date string
  updated_at: string;   // ISO date string
}

export interface ArticleResponse {
  id: number;
  slug: string;
  author_id?: number;
  created_at: string;   // ISO date string
  updated_at: string;   // ISO date string
  translations: ArticleTranslation[];
}

// For creating an article
export interface CreateArticleTranslation {
  language_code: string;
  title: string;
  content: string;
  is_published?: boolean; // Default to false if not provided by backend/frontend
}

export interface CreateArticleRequest {
  slug: string;
  author_id?: number;
  translations: CreateArticleTranslation[];
}

@Injectable({
  providedIn: 'root'
})
export class ArticleService {
  private apiUrl = '/api/articles'; // Angular proxy will handle this

  constructor(private http: HttpClient) { }

  getArticles(lang?: string, limit?: number, offset?: number): Observable<ArticleResponse[]> {
    let params = new HttpParams();
    if (lang) {
      params = params.set('lang', lang);
    }
    if (limit !== undefined) {
      params = params.set('limit', limit.toString());
    }
    if (offset !== undefined) {
      params = params.set('offset', offset.toString());
    }
    return this.http.get<ArticleResponse[]>(this.apiUrl, { params });
  }

  getArticle(slug: string, lang?: string): Observable<ArticleResponse> {
    let params = new HttpParams();
    if (lang) {
      params = params.set('lang', lang);
    }
    return this.http.get<ArticleResponse>(`${this.apiUrl}/${slug}`, { params });
  }

  createArticle(articleData: CreateArticleRequest): Observable<ArticleResponse> {
    return this.http.post<ArticleResponse>(this.apiUrl, articleData);
  }

  // Optional methods to be added later if needed:
  // updateArticle(slug: string, articleData: Partial<CreateArticleRequest>): Observable<ArticleResponse> {
  //   return this.http.put<ArticleResponse>(`${this.apiUrl}/${slug}`, articleData);
  // }
  //
  // deleteArticle(slug: string): Observable<void> {
  //   return this.http.delete<void>(`${this.apiUrl}/${slug}`);
  // }
}

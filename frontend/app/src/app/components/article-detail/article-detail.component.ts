import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common'; // For *ngIf, etc.
import { ActivatedRoute, RouterModule } from '@angular/router'; // For route params and routerLink
import { ArticleService, ArticleResponse, ArticleTranslation } from '../../services/article.service';
import { Observable, switchMap } from 'rxjs';

@Component({
  selector: 'app-article-detail',
  standalone: true,
  imports: [CommonModule, RouterModule],
  templateUrl: './article-detail.component.html',
  styleUrls: ['./article-detail.component.scss']
})
export class ArticleDetailComponent implements OnInit {
  article: ArticleResponse | null = null;
  isLoading = true;
  error: string | null = null;
  selectedLanguage: string | undefined; // To potentially hold language from query param

  constructor(
    private route: ActivatedRoute,
    private articleService: ArticleService
  ) { }

  ngOnInit(): void {
    this.route.paramMap.pipe(
      switchMap(params => {
        const slug = params.get('slug');
        if (!slug) {
          this.isLoading = false;
          this.error = 'Article slug not found in URL.';
          throw new Error('Slug is required'); // Or handle more gracefully
        }
        
        // Check for 'lang' query parameter
        this.selectedLanguage = this.route.snapshot.queryParamMap.get('lang') || undefined;
        
        this.isLoading = true;
        return this.articleService.getArticle(slug, this.selectedLanguage);
      })
    ).subscribe({
      next: (data) => {
        this.article = data;
        this.isLoading = false;
      },
      error: (err) => {
        console.error('Error fetching article:', err);
        if (err.status === 404) {
          this.error = 'Article not found.';
        } else {
          this.error = 'Failed to load article. Please try again later.';
        }
        this.isLoading = false;
      }
    });
  }

  // Helper to get the first translation or a specific one if language is matched
  getDisplayTranslation(): ArticleTranslation | null {
    if (!this.article || !this.article.translations || this.article.translations.length === 0) {
      return null;
    }
    // If a language was selected and a matching translation exists, return it
    if (this.selectedLanguage) {
      const specificTranslation = this.article.translations.find(t => t.language_code === this.selectedLanguage);
      if (specificTranslation) return specificTranslation;
    }
    // Otherwise, return the first translation
    return this.article.translations[0];
  }
}

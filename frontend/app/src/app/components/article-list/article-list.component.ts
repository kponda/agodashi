import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common'; // Needed for *ngFor, *ngIf, etc.
import { RouterModule } from '@angular/router'; // Needed for routerLink
import { ArticleService, ArticleResponse } from '../../services/article.service';

@Component({
  selector: 'app-article-list',
  standalone: true,
  imports: [CommonModule, RouterModule], // Import CommonModule and RouterModule
  templateUrl: './article-list.component.html',
  styleUrls: ['./article-list.component.scss']
})
export class ArticleListComponent implements OnInit {
  articles: ArticleResponse[] = [];
  isLoading = true;
  error: string | null = null;

  constructor(private articleService: ArticleService) { }

  ngOnInit(): void {
    // Fetch articles with a default language, e.g., 'en'
    // The backend listArticlesHandler defaults to 'en' if no lang is specified.
    this.articleService.getArticles(undefined, 10, 0).subscribe({
      next: (data) => {
        this.articles = data;
        this.isLoading = false;
      },
      error: (err) => {
        console.error('Error fetching articles:', err);
        this.error = 'Failed to load articles. Please try again later.';
        this.isLoading = false;
      }
    });
  }

  // Helper to get a display title
  getDisplayTitle(article: ArticleResponse): string {
    if (article.translations && article.translations.length > 0) {
      return article.translations[0].title; // Display title from the first translation
    }
    return 'N/A'; // Or some default if no translations
  }
}

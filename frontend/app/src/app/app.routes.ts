import { Routes } from '@angular/router';
import { ArticleListComponent } from './components/article-list/article-list.component';
import { ArticleDetailComponent } from './components/article-detail/article-detail.component';
import { ArticleCreateComponent } from './components/article-create/article-create.component';

export const routes: Routes = [
  { path: '', redirectTo: '/articles', pathMatch: 'full' },
  { path: 'articles', component: ArticleListComponent },
  { path: 'articles/new', component: ArticleCreateComponent }, // 'new' before ':slug'
  { path: 'articles/:slug', component: ArticleDetailComponent },
  // Potentially a wildcard route for 404s later:
  // { path: '**', component: PageNotFoundComponent }, // Example for future 404
];

package compose.project.mudrodnabe.repository;

import compose.project.mudrodnabe.entities.Quote;
import org.springframework.data.mongodb.repository.MongoRepository;
import org.springframework.stereotype.Repository;

@Repository
public interface QuoteRepository extends MongoRepository<Quote, String> {}

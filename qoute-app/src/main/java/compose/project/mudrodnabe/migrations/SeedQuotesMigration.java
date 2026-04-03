package compose.project.mudrodnabe.migrations;

import compose.project.mudrodnabe.entities.Quote;
import io.mongock.api.annotations.ChangeUnit;
import io.mongock.api.annotations.Execution;
import io.mongock.api.annotations.RollbackExecution;
import org.springframework.data.mongodb.core.MongoTemplate;

import java.util.List;

@ChangeUnit(id = "seed-quotes", order = "001", author = "dariushorvath")
public class SeedQuotesMigration {
    @Execution
    public void seedQuotes(MongoTemplate mongoTemplate) {
        List<Quote> quotes = List.of(
                new Quote(null, "The only way to do great work is to love what you do."),
                new Quote(null, "In the middle of every difficulty lies opportunity."),
                new Quote(null, "It does not matter how slowly you go as long as you do not stop.")
        );

        mongoTemplate.insertAll(quotes);
    }

    @RollbackExecution
    public void rollback(MongoTemplate mongoTemplate) {
        mongoTemplate.dropCollection(Quote.class);
    }
}


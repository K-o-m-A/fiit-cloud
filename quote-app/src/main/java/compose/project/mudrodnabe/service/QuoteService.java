package compose.project.mudrodnabe.service;

import compose.project.mudrodnabe.domain.QuoteDto;
import compose.project.mudrodnabe.entities.Quote;
import compose.project.mudrodnabe.repository.QuoteRepository;
import lombok.AllArgsConstructor;
import org.springframework.stereotype.Service;

import java.util.List;
import java.util.Random;

@Service
@AllArgsConstructor
public class QuoteService {

    private final QuoteRepository quoteRepository;

    public QuoteDto getQuote() {
        List<Quote> quotes = quoteRepository.findAll();

        if (quotes.isEmpty()) {
            return new QuoteDto("No quotes found");
        }

        // pick a random one
        Random random = new Random();
        Quote randomQuote = quotes.get(random.nextInt(quotes.size()));

        return new QuoteDto(randomQuote.getQuote());
    }
}